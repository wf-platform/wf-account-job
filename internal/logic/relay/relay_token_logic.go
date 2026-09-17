package relay

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"wf-account-job/ent/predicate"
	"wf-account-job/ent/relaytoken"
	"wf-account-job/internal/svc"
	"wf-account-job/internal/utils/dberrorhandler"
	"wf-account-job/types/job"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRelayTokenListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRelayTokenListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRelayTokenListLogic {
	return &GetRelayTokenListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRelayTokenListLogic) GetRelayTokenList(in *job.RelayTokenListReq) (*job.RelayTokenListResp, error) {
	var predicates []predicate.RelayToken
	if len(in.ChainIds) > 0 {
		predicates = append(predicates, relaytoken.ChainIDIn(in.ChainIds...))
	}
	if len(in.Ids) > 0 {
		predicates = append(predicates, relaytoken.TokenIDIn(in.Ids...))
	}
	if in.Name != nil {
		predicates = append(predicates, relaytoken.NameContains(*in.Name))
	}
	if in.Symbol != nil {
		predicates = append(predicates, relaytoken.SymbolContains(*in.Symbol))
	}
	if in.Address != nil {
		predicates = append(predicates, relaytoken.AddressEQ(*in.Address))
	}
	if in.Supported != nil {
		predicates = append(predicates, relaytoken.SupportedEQ(*in.Supported))
	}

	query := l.svcCtx.DB.RelayToken.Query().Where(predicates...)
	countQuery := query.Clone()
	total, err := countQuery.Count(l.ctx)
	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	page := in.Page
	pageSize := in.PageSize
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 100
	}

	list, err := query.
		Order(relaytoken.ByChainID(sql.OrderAsc()), relaytoken.ByTokenID(sql.OrderAsc())).
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		All(l.ctx)
	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	resp := &job.RelayTokenListResp{
		Total: uint64(total),
		Data:  make([]*job.RelayTokenInfo, 0, len(list)),
	}
	for _, v := range list {
		resp.Data = append(resp.Data, toRelayTokenInfo(v))
	}

	return resp, nil
}

type GetRelayTokenByChainAndIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRelayTokenByChainAndIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRelayTokenByChainAndIdLogic {
	return &GetRelayTokenByChainAndIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRelayTokenByChainAndIdLogic) GetRelayTokenByChainAndId(in *job.RelayTokenKeyReq) (*job.RelayTokenInfo, error) {
	result, err := l.svcCtx.DB.RelayToken.Query().
		Where(relaytoken.ChainIDEQ(in.ChainId), relaytoken.TokenIDEQ(in.Id)).
		Only(l.ctx)
	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	return toRelayTokenInfo(result), nil
}
