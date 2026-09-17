package relay

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"wf-account-job/ent/predicate"
	"wf-account-job/ent/relaychain"
	"wf-account-job/ent/relaytoken"
	"wf-account-job/internal/svc"
	"wf-account-job/internal/utils/dberrorhandler"
	"wf-account-job/types/job"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClientRelayChainListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClientRelayChainListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClientRelayChainListLogic {
	return &GetClientRelayChainListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetClientRelayChainListLogic) GetClientRelayChainList(in *job.Empty) (*job.ClientRelayChainListResp, error) {
	list, err := l.svcCtx.DB.RelayChain.Query().
		Where(relaychain.EnabledEQ(true), relaychain.SupportedEQ(true)).
		Order(relaychain.ByID(sql.OrderAsc())).
		All(l.ctx)
	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	resp := &job.ClientRelayChainListResp{
		Data: make([]*job.ClientRelayChainInfo, 0, len(list)),
	}
	for _, v := range list {
		resp.Data = append(resp.Data, toClientRelayChainInfo(v))
	}

	return resp, nil
}

type GetClientRelayTokenListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClientRelayTokenListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClientRelayTokenListLogic {
	return &GetClientRelayTokenListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetClientRelayTokenListLogic) GetClientRelayTokenList(in *job.ClientRelayTokenListReq) (*job.ClientRelayTokenListResp, error) {
	predicates := []predicate.RelayToken{
		relaytoken.SupportedEQ(true),
		relaytoken.HasChainWith(relaychain.EnabledEQ(true), relaychain.SupportedEQ(true)),
	}
	if in.ChainId > 0 {
		predicates = append(predicates, relaytoken.ChainIDEQ(in.ChainId))
	}
	if len(in.Ids) > 0 {
		predicates = append(predicates, relaytoken.TokenIDIn(in.Ids...))
	}

	list, err := l.svcCtx.DB.RelayToken.Query().
		Where(predicates...).
		Order(relaytoken.ByChainID(sql.OrderAsc()), relaytoken.ByTokenID(sql.OrderAsc())).
		All(l.ctx)
	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	resp := &job.ClientRelayTokenListResp{
		Data: make([]*job.ClientRelayTokenInfo, 0, len(list)),
	}
	for _, v := range list {
		resp.Data = append(resp.Data, toClientRelayTokenInfo(v))
	}

	return resp, nil
}
