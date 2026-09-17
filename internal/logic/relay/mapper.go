package relay

import (
	"wf-account-job/ent"
	"wf-account-job/types/job"

	"github.com/suyuan32/simple-admin-common/utils/pointy"
)

func toRelayChainInfo(v *ent.RelayChain) *job.RelayChainInfo {
	if v == nil {
		return nil
	}

	return &job.RelayChainInfo{
		Id:             v.ID,
		CreatedAt:      pointy.GetPointer(v.CreatedAt.UnixMilli()),
		UpdatedAt:      pointy.GetPointer(v.UpdatedAt.UnixMilli()),
		Slug:           &v.Slug,
		Name:           &v.Name,
		LogoUrl:        &v.LogoURL,
		Type:           &v.Type,
		VmType:         &v.VMType,
		Protocol:       &v.Protocol,
		BaseChainId:    &v.BaseChainID,
		ExplorerUrl:    &v.ExplorerURL,
		ExplorerName:   &v.ExplorerName,
		RpcUrl:         &v.RPCURL,
		WsRpcUrl:       &v.WsRPCURL,
		NativeSymbol:   &v.NativeSymbol,
		NativeDecimals: pointy.GetPointer(int64(v.NativeDecimals)),
		DepositEnabled: &v.DepositEnabled,
		TokenSupport:   &v.TokenSupport,
		Disabled:       &v.Disabled,
		Supported:      &v.Supported,
		Enabled:        &v.Enabled,
		RawData:        &v.RawData,
	}
}

func toRelayTokenInfo(v *ent.RelayToken) *job.RelayTokenInfo {
	if v == nil {
		return nil
	}

	return &job.RelayTokenInfo{
		Id:               v.TokenID,
		ChainId:          v.ChainID,
		CreatedAt:        pointy.GetPointer(v.CreatedAt.UnixMilli()),
		UpdatedAt:        pointy.GetPointer(v.UpdatedAt.UnixMilli()),
		Address:          &v.Address,
		Name:             &v.Name,
		Symbol:           &v.Symbol,
		LogoUrl:          &v.LogoURL,
		Decimals:         pointy.GetPointer(int64(v.Decimals)),
		Native:           &v.Native,
		Stablecoin:       &v.Stablecoin,
		SupportsBridging: &v.SupportsBridging,
		SupportsPermit:   &v.SupportsPermit,
		IsFeatured:       &v.IsFeatured,
		IsSolver:         &v.IsSolver,
		IsErc20:          &v.IsErc20,
		Supported:        &v.Supported,
		RawData:          &v.RawData,
	}
}

func toClientRelayChainInfo(v *ent.RelayChain) *job.ClientRelayChainInfo {
	if v == nil {
		return nil
	}

	return &job.ClientRelayChainInfo{
		Id:      v.ID,
		Name:    v.Name,
		LogoUrl: v.LogoURL,
	}
}

func toClientRelayTokenInfo(v *ent.RelayToken) *job.ClientRelayTokenInfo {
	if v == nil {
		return nil
	}

	return &job.ClientRelayTokenInfo{
		Id:       v.TokenID,
		ChainId:  v.ChainID,
		Name:     v.Name,
		Symbol:   v.Symbol,
		LogoUrl:  v.LogoURL,
		Address:  v.Address,
		Decimals: int64(v.Decimals),
	}
}
