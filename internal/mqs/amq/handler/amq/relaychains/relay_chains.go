package relaychains

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/suyuan32/simple-admin-common/i18n"
	"github.com/zeromicro/go-zero/core/errorx"
	"github.com/zeromicro/go-zero/core/logx"

	"wf-account-job/ent"
	"wf-account-job/ent/relaychain"
	"wf-account-job/ent/relaytoken"
	"wf-account-job/ent/task"
	"wf-account-job/internal/enum/taskresult"
	"wf-account-job/internal/mqs/amq/types/pattern"
	"wf-account-job/internal/svc"
	"wf-account-job/internal/utils/dberrorhandler"
)

const relayChainsURL = "https://api.relay.link/chains"
const relayNativeZeroAddress = "0x0000000000000000000000000000000000000000"

type Handler struct {
	svcCtx *svc.ServiceContext
	taskID uint64
	client *http.Client
}

type chainData struct {
	id             int64
	slug           string
	name           string
	logoURL        string
	chainType      string
	vmType         string
	protocol       string
	baseChainID    string
	explorerURL    string
	explorerName   string
	rpcURL         string
	wsRPCURL       string
	nativeSymbol   string
	nativeDecimals int
	depositEnabled bool
	tokenSupport   string
	disabled       bool
	supported      bool
	enabled        bool
	rawData        string
}

type tokenData struct {
	tokenID          string
	chainID          int64
	address          string
	name             string
	symbol           string
	logoURL          string
	decimals         int
	native           bool
	stablecoin       bool
	supportsBridging bool
	supportsPermit   bool
	isFeatured       bool
	isSolver         bool
	isERC20          bool
	supported        bool
	rawData          string
}

type relayTokenSnapshot struct {
	token tokenData
	raw   map[string]interface{}
}

func NewHandler(svcCtx *svc.ServiceContext) *Handler {
	return &Handler{
		svcCtx: svcCtx,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ProcessTask if return err != nil, asynq will retry.
func (h *Handler) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	taskID, err := h.ensureTaskID(ctx)
	if err != nil {
		logx.Errorw("failed to load relay chains task info")
		return errorx.NewInternalError(i18n.DatabaseError)
	}

	startTime := time.Now()
	result := taskresult.Success

	err = h.sync(ctx)
	if err != nil {
		result = taskresult.Failed
	}

	finishTime := time.Now()
	logErr := h.svcCtx.DB.TaskLog.Create().
		SetStartedAt(startTime).
		SetFinishedAt(finishTime).
		SetResult(result).
		SetTasksID(taskID).
		Exec(context.Background())
	if logErr != nil {
		return dberrorhandler.DefaultEntError(logx.WithContext(context.Background()), logErr,
			"failed to save relay chains task log to database")
	}

	return err
}

func (h *Handler) ensureTaskID(ctx context.Context) (uint64, error) {
	if h.taskID != 0 {
		return h.taskID, nil
	}

	taskInfo, err := h.svcCtx.DB.Task.Query().Where(task.PatternEQ(pattern.RelayChains)).First(ctx)
	if err != nil {
		return 0, err
	}
	h.taskID = taskInfo.ID
	return h.taskID, nil
}

func (h *Handler) sync(ctx context.Context) error {
	logx.WithContext(ctx).Infow("relay chains sync started")

	chains, err := h.fetch(ctx)
	if err != nil {
		return err
	}

	successCount := 0
	failCount := 0
	fetchedChainIDs := make([]int64, 0, len(chains))

	for i, rawChain := range chains {
		chainMap, ok := rawChain.(map[string]interface{})
		if !ok {
			logx.WithContext(ctx).Errorw("invalid relay chain data format", logx.Field("index", i+1))
			failCount++
			continue
		}

		chain, err := buildRelayChain(chainMap)
		if err != nil {
			logx.WithContext(ctx).Errorw("failed to build relay chain", logx.Field("index", i+1), logx.Field("detail", err.Error()))
			failCount++
			continue
		}
		fetchedChainIDs = append(fetchedChainIDs, chain.id)

		tx, err := h.svcCtx.DB.Tx(ctx)
		if err != nil {
			logx.WithContext(ctx).Errorw("failed to begin relay chain transaction", logx.Field("chain_id", chain.id), logx.Field("detail", err.Error()))
			failCount++
			continue
		}

		tokenCount, err := h.syncChain(ctx, tx, chainMap, chain)
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				logx.WithContext(ctx).Errorw("failed to rollback relay chain transaction", logx.Field("chain_id", chain.id), logx.Field("detail", rollbackErr.Error()))
			}
			logx.WithContext(ctx).Errorw("failed to sync relay chain", logx.Field("chain_id", chain.id), logx.Field("detail", err.Error()))
			failCount++
			continue
		}

		if err := tx.Commit(); err != nil {
			logx.WithContext(ctx).Errorw("failed to commit relay chain transaction", logx.Field("chain_id", chain.id), logx.Field("detail", err.Error()))
			failCount++
			continue
		}

		successCount++
		if i < 5 {
			logx.WithContext(ctx).Infow("relay chain synced",
				logx.Field("chain_id", chain.id),
				logx.Field("name", chain.name),
				logx.Field("supported", chain.supported),
				logx.Field("token_count", tokenCount))
		}
	}

	if err := markMissingChainsUnsupported(ctx, h.svcCtx.DB, fetchedChainIDs); err != nil {
		return err
	}

	if err := markTokensByChainSupport(ctx, h.svcCtx.DB); err != nil {
		return err
	}

	logx.WithContext(ctx).Infow("relay chains sync completed",
		logx.Field("success", successCount),
		logx.Field("failed", failCount),
		logx.Field("total", len(chains)))

	if failCount > 0 {
		return fmt.Errorf("completed with %d failures out of %d chains", failCount, len(chains))
	}

	return nil
}

func (h *Handler) fetch(ctx context.Context) ([]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, relayChainsURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	chains, ok := payload["chains"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no chains data found in response")
	}

	return chains, nil
}

func (h *Handler) syncChain(ctx context.Context, tx *ent.Tx, chainMap map[string]interface{}, chain chainData) (int, error) {
	if err := upsertRelayChain(ctx, tx, chain); err != nil {
		return 0, err
	}

	return syncRelayTokens(ctx, tx, chainMap, chain.id, chain.supported)
}

func buildRelayChain(chainMap map[string]interface{}) (chainData, error) {
	relayID, err := getChainID(chainMap)
	if err != nil {
		return chainData{}, err
	}

	slug := getStringValue(chainMap, "name")
	name := getFirstStringValue(chainMap, "displayName", "name")
	if name == "" {
		return chainData{}, fmt.Errorf("skipping due to empty name")
	}

	vmType := getStringValue(chainMap, "vmType")
	protocol := getStringValue(chainMap, "protocol")
	chainType := firstNonEmpty(vmType, protocol, getStringValue(chainMap, "type"), "evm")
	disabled := getBoolValue(chainMap, "disabled")

	chain := chainData{
		id:             relayID,
		slug:           strings.TrimSpace(slug),
		name:           strings.TrimSpace(name),
		logoURL:        strings.TrimSpace(getStringValue(chainMap, "iconUrl")),
		chainType:      strings.TrimSpace(chainType),
		vmType:         strings.TrimSpace(vmType),
		protocol:       strings.TrimSpace(protocol),
		baseChainID:    strings.TrimSpace(getStringValue(chainMap, "baseChainId")),
		explorerURL:    strings.TrimSpace(getStringValue(chainMap, "explorerUrl")),
		explorerName:   strings.TrimSpace(getStringValue(chainMap, "explorerName")),
		rpcURL:         strings.TrimSpace(getStringValue(chainMap, "httpRpcUrl")),
		wsRPCURL:       strings.TrimSpace(getStringValue(chainMap, "wsRpcUrl")),
		depositEnabled: getBoolValue(chainMap, "depositEnabled"),
		tokenSupport:   strings.TrimSpace(getStringValue(chainMap, "tokenSupport")),
		disabled:       disabled,
		supported:      !disabled,
		enabled:        true,
	}

	if currency, exists := chainMap["currency"]; exists {
		if currencyMap, ok := currency.(map[string]interface{}); ok {
			chain.nativeSymbol = strings.TrimSpace(getStringValue(currencyMap, "symbol"))
			chain.nativeDecimals = getIntValue(currencyMap, "decimals")
		}
	}

	rawJSON, _ := json.Marshal(chainMap)
	chain.rawData = string(rawJSON)

	if chain.chainType == "" {
		chain.chainType = "evm"
	}

	return chain, nil
}

func upsertRelayChain(ctx context.Context, tx *ent.Tx, chain chainData) error {
	existing, err := tx.RelayChain.Query().Where(relaychain.IDEQ(chain.id)).Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return err
		}

		return tx.RelayChain.Create().
			SetID(chain.id).
			SetSlug(chain.slug).
			SetName(chain.name).
			SetLogoURL(chain.logoURL).
			SetType(chain.chainType).
			SetVMType(chain.vmType).
			SetProtocol(chain.protocol).
			SetBaseChainID(chain.baseChainID).
			SetExplorerURL(chain.explorerURL).
			SetExplorerName(chain.explorerName).
			SetRPCURL(chain.rpcURL).
			SetWsRPCURL(chain.wsRPCURL).
			SetNativeSymbol(chain.nativeSymbol).
			SetNativeDecimals(chain.nativeDecimals).
			SetDepositEnabled(chain.depositEnabled).
			SetTokenSupport(chain.tokenSupport).
			SetDisabled(chain.disabled).
			SetSupported(chain.supported).
			SetEnabled(chain.enabled).
			SetRawData(chain.rawData).
			Exec(ctx)
	}

	return tx.RelayChain.UpdateOne(existing).
		SetSlug(chain.slug).
		SetName(chain.name).
		SetLogoURL(chain.logoURL).
		SetType(chain.chainType).
		SetVMType(chain.vmType).
		SetProtocol(chain.protocol).
		SetBaseChainID(chain.baseChainID).
		SetExplorerURL(chain.explorerURL).
		SetExplorerName(chain.explorerName).
		SetRPCURL(chain.rpcURL).
		SetWsRPCURL(chain.wsRPCURL).
		SetNativeSymbol(chain.nativeSymbol).
		SetNativeDecimals(chain.nativeDecimals).
		SetDepositEnabled(chain.depositEnabled).
		SetTokenSupport(chain.tokenSupport).
		SetDisabled(chain.disabled).
		SetSupported(chain.supported).
		SetRawData(chain.rawData).
		Exec(ctx)
}

func syncRelayTokens(ctx context.Context, tx *ent.Tx, chainMap map[string]interface{}, chainID int64, chainSupported bool) (int, error) {
	if !chainSupported {
		_, err := tx.RelayToken.Update().Where(relaytoken.ChainIDEQ(chainID)).SetSupported(false).Save(ctx)
		return 0, err
	}

	tokens, ids := buildRelayTokens(chainMap, chainID, chainSupported)
	if len(tokens) == 0 {
		_, err := tx.RelayToken.Update().Where(relaytoken.ChainIDEQ(chainID)).SetSupported(false).Save(ctx)
		return 0, err
	}

	successCount := 0
	for _, token := range tokens {
		if err := upsertRelayToken(ctx, tx, token); err != nil {
			return successCount, err
		}
		successCount++
	}

	if err := markMissingTokensUnsupported(ctx, tx, chainID, ids); err != nil {
		return successCount, err
	}

	return successCount, nil
}

func buildRelayTokens(chainMap map[string]interface{}, chainID int64, chainSupported bool) ([]tokenData, []string) {
	merged := make(map[string]*relayTokenSnapshot)

	mergeRelayTokenList(merged, chainMap, "featuredTokens", chainID, chainSupported, func(token *tokenData) {
		token.isFeatured = true
	})
	mergeRelayTokenList(merged, chainMap, "solverCurrencies", chainID, chainSupported, func(token *tokenData) {
		token.isSolver = true
	})
	mergeRelayTokenList(merged, chainMap, "erc20Currencies", chainID, chainSupported, func(token *tokenData) {
		token.isERC20 = true
	})

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	tokens := make([]tokenData, 0, len(keys))
	for _, key := range keys {
		snapshot := merged[key]
		snapshot.raw["sourceLists"] = map[string]bool{
			"featuredTokens":   snapshot.token.isFeatured,
			"solverCurrencies": snapshot.token.isSolver,
			"erc20Currencies":  snapshot.token.isERC20,
		}
		rawJSON, _ := json.Marshal(snapshot.raw)
		snapshot.token.rawData = string(rawJSON)
		tokens = append(tokens, snapshot.token)
	}

	return tokens, keys
}

func mergeRelayTokenList(
	merged map[string]*relayTokenSnapshot,
	chainMap map[string]interface{},
	listKey string,
	chainID int64,
	chainSupported bool,
	applyFlags func(token *tokenData),
) {
	listRaw, exists := chainMap[listKey]
	if !exists {
		return
	}

	list, ok := listRaw.([]interface{})
	if !ok {
		return
	}

	for _, item := range list {
		tokenMap, ok := item.(map[string]interface{})
		if !ok || !isEffectiveRelayToken(tokenMap, chainSupported) {
			continue
		}

		token := buildRelayToken(chainID, tokenMap, chainSupported)
		if token.tokenID == "" || token.address == "" || token.symbol == "" {
			continue
		}

		snapshot, exists := merged[token.tokenID]
		if !exists {
			snapshot = &relayTokenSnapshot{
				token: token,
				raw:   cloneMap(tokenMap),
			}
			merged[token.tokenID] = snapshot
		} else {
			mergeRelayToken(&snapshot.token, token)
			mergeIntoMap(snapshot.raw, tokenMap)
		}

		applyFlags(&snapshot.token)
	}
}

func buildRelayToken(chainID int64, tokenMap map[string]interface{}, chainSupported bool) tokenData {
	address := normalizeAddress(getStringValue(tokenMap, "address"))
	tokenID := strings.TrimSpace(getStringValue(tokenMap, "id"))
	if tokenID == "" {
		tokenID = address
	}

	token := tokenData{
		tokenID:          tokenID,
		chainID:          chainID,
		address:          address,
		name:             strings.TrimSpace(getStringValue(tokenMap, "name")),
		symbol:           strings.TrimSpace(getStringValue(tokenMap, "symbol")),
		decimals:         getIntValue(tokenMap, "decimals"),
		native:           isNativeToken(address),
		supportsBridging: getBoolValue(tokenMap, "supportsBridging"),
		supportsPermit:   getBoolValue(tokenMap, "supportsPermit"),
		supported:        chainSupported,
	}

	if metadataRaw, exists := tokenMap["metadata"]; exists {
		if metadataMap, ok := metadataRaw.(map[string]interface{}); ok {
			token.logoURL = strings.TrimSpace(getStringValue(metadataMap, "logoURI"))
		}
	}

	return token
}

func isEffectiveRelayToken(tokenMap map[string]interface{}, chainSupported bool) bool {
	if !chainSupported {
		return false
	}
	if disabled, exists := getOptionalBoolValue(tokenMap, "disabled"); exists && disabled {
		return false
	}
	if enabled, exists := getOptionalBoolValue(tokenMap, "enabled"); exists && !enabled {
		return false
	}
	if supported, exists := getOptionalBoolValue(tokenMap, "supported"); exists && !supported {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(getStringValue(tokenMap, "status"))) {
	case "disabled", "inactive", "unsupported", "deprecated":
		return false
	}
	return true
}

func mergeRelayToken(dst *tokenData, src tokenData) {
	if dst.address == "" {
		dst.address = src.address
	}
	if dst.name == "" {
		dst.name = src.name
	}
	if dst.symbol == "" {
		dst.symbol = src.symbol
	}
	if dst.logoURL == "" {
		dst.logoURL = src.logoURL
	}
	if dst.decimals == 0 && src.decimals != 0 {
		dst.decimals = src.decimals
	}
	dst.native = dst.native || src.native
	dst.stablecoin = dst.stablecoin || src.stablecoin
	dst.supportsBridging = dst.supportsBridging || src.supportsBridging
	dst.supportsPermit = dst.supportsPermit || src.supportsPermit
	dst.supported = dst.supported || src.supported
}

func upsertRelayToken(ctx context.Context, tx *ent.Tx, token tokenData) error {
	existing, err := tx.RelayToken.Query().
		Where(relaytoken.TokenIDEQ(token.tokenID), relaytoken.ChainIDEQ(token.chainID)).
		Only(ctx)
	if err == nil {
		return updateRelayToken(ctx, tx, existing, token)
	}
	if !ent.IsNotFound(err) {
		return err
	}

	err = tx.RelayToken.Create().
		SetTokenID(token.tokenID).
		SetChainID(token.chainID).
		SetAddress(token.address).
		SetName(token.name).
		SetSymbol(token.symbol).
		SetLogoURL(token.logoURL).
		SetDecimals(token.decimals).
		SetNative(token.native).
		SetStablecoin(token.stablecoin).
		SetSupportsBridging(token.supportsBridging).
		SetSupportsPermit(token.supportsPermit).
		SetIsFeatured(token.isFeatured).
		SetIsSolver(token.isSolver).
		SetIsErc20(token.isERC20).
		SetSupported(token.supported).
		SetRawData(token.rawData).
		Exec(ctx)
	if err == nil {
		return nil
	}

	byAddress, addressErr := tx.RelayToken.Query().
		Where(relaytoken.ChainIDEQ(token.chainID), relaytoken.AddressEQ(token.address)).
		Only(ctx)
	if addressErr == nil {
		logx.WithContext(ctx).Errorw("relay token id mismatch, keeping existing id",
			logx.Field("chain_id", token.chainID),
			logx.Field("address", token.address),
			logx.Field("existing_token_id", byAddress.TokenID),
			logx.Field("incoming_token_id", token.tokenID))
		token.tokenID = byAddress.TokenID
		return updateRelayToken(ctx, tx, byAddress, token)
	}

	if ent.IsNotFound(addressErr) {
		return err
	}
	return addressErr
}

func updateRelayToken(ctx context.Context, tx *ent.Tx, existing *ent.RelayToken, token tokenData) error {
	return tx.RelayToken.UpdateOne(existing).
		SetAddress(token.address).
		SetName(token.name).
		SetSymbol(token.symbol).
		SetLogoURL(token.logoURL).
		SetDecimals(token.decimals).
		SetNative(token.native).
		SetStablecoin(token.stablecoin).
		SetSupportsBridging(token.supportsBridging).
		SetSupportsPermit(token.supportsPermit).
		SetIsFeatured(token.isFeatured).
		SetIsSolver(token.isSolver).
		SetIsErc20(token.isERC20).
		SetSupported(token.supported).
		SetRawData(token.rawData).
		Exec(ctx)
}

func markMissingTokensUnsupported(ctx context.Context, tx *ent.Tx, chainID int64, currentIDs []string) error {
	update := tx.RelayToken.Update().Where(relaytoken.ChainIDEQ(chainID))
	if len(currentIDs) > 0 {
		update.Where(relaytoken.TokenIDNotIn(currentIDs...))
	}
	_, err := update.SetSupported(false).Save(ctx)
	return err
}

func markMissingChainsUnsupported(ctx context.Context, db *ent.Client, chainIDs []int64) error {
	if len(chainIDs) == 0 {
		return nil
	}
	_, err := db.RelayChain.Update().
		Where(relaychain.IDNotIn(chainIDs...)).
		SetSupported(false).
		Save(ctx)
	return err
}

func markTokensByChainSupport(ctx context.Context, db *ent.Client) error {
	unsupportedChainIDs, err := db.RelayChain.Query().
		Where(relaychain.SupportedEQ(false)).
		IDs(ctx)
	if err != nil {
		return err
	}
	if len(unsupportedChainIDs) == 0 {
		return nil
	}

	_, err = db.RelayToken.Update().
		Where(relaytoken.ChainIDIn(unsupportedChainIDs...)).
		SetSupported(false).
		Save(ctx)
	return err
}

func getStringValue(m map[string]interface{}, key string, defaultValue ...string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
		switch v := val.(type) {
		case int:
			return fmt.Sprintf("%d", v)
		case int64:
			return fmt.Sprintf("%d", v)
		case float64:
			if v == float64(int64(v)) {
				return fmt.Sprintf("%d", int64(v))
			}
			return fmt.Sprintf("%v", v)
		case float32:
			if v == float32(int32(v)) {
				return fmt.Sprintf("%d", int32(v))
			}
			return fmt.Sprintf("%v", v)
		case json.Number:
			return v.String()
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func getFirstStringValue(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val := getStringValue(m, key); val != "" {
			return val
		}
	}
	return ""
}

func getChainID(chainMap map[string]interface{}) (int64, error) {
	for _, key := range []string{"id", "chainId", "chain_id", "externalId", "external_id"} {
		if rawID := getStringValue(chainMap, key); rawID != "" {
			relayID, err := parseInt64String(rawID)
			if err != nil {
				return 0, fmt.Errorf("invalid relay chain id %q: %w", rawID, err)
			}
			return relayID, nil
		}
	}
	return 0, errors.New("skipping due to empty id")
}

func getIntValue(m map[string]interface{}, key string, defaultValue ...int) int {
	if val, exists := m[key]; exists {
		switch v := val.(type) {
		case float64:
			return int(v)
		case float32:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		case json.Number:
			if i, err := v.Int64(); err == nil {
				return int(i)
			}
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return 0
}

func getBoolValue(m map[string]interface{}, key string, defaultValue ...bool) bool {
	if val, exists := m[key]; exists {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			lower := strings.ToLower(strings.TrimSpace(v))
			return lower == "true" || lower == "1" || lower == "yes"
		case float64:
			return v != 0
		case int:
			return v != 0
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return false
}

func getOptionalBoolValue(m map[string]interface{}, key string) (bool, bool) {
	if _, exists := m[key]; !exists {
		return false, false
	}
	return getBoolValue(m, key), true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func isNativeToken(address string) bool {
	return address == relayNativeZeroAddress || address == "native"
}

func parseInt64String(value string) (int64, error) {
	result, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	if result <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return result, nil
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func mergeIntoMap(dst, src map[string]interface{}) {
	for key, value := range src {
		dst[key] = value
	}
}
