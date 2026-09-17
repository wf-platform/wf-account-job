package relay

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/wf-platform/wf-account-job/ent/predicate"
	"github.com/wf-platform/wf-account-job/ent/relaychain"
	"github.com/wf-platform/wf-account-job/internal/svc"
	"github.com/wf-platform/wf-account-job/internal/utils/dberrorhandler"
	"github.com/wf-platform/wf-account-job/types/job"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRelayChainListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRelayChainListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRelayChainListLogic {
	return &GetRelayChainListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRelayChainListLogic) GetRelayChainList(in *job.RelayChainListReq) (*job.RelayChainListResp, error) {
	var predicates []predicate.RelayChain
	if len(in.Ids) > 0 {
		predicates = append(predicates, relaychain.IDIn(in.Ids...))
	}
	if in.Name != nil {
		predicates = append(predicates, relaychain.NameContains(*in.Name))
	}
	if in.Type != nil {
		predicates = append(predicates, relaychain.TypeEQ(*in.Type))
	}
	if in.Enabled != nil {
		predicates = append(predicates, relaychain.EnabledEQ(*in.Enabled))
	}
	if in.Supported != nil {
		predicates = append(predicates, relaychain.SupportedEQ(*in.Supported))
	}

	query := l.svcCtx.DB.RelayChain.Query().Where(predicates...)
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
		Order(relaychain.ByID(sql.OrderAsc())).
		Offset(int((page - 1) * pageSize)).
		Limit(int(pageSize)).
		All(l.ctx)
	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	resp := &job.RelayChainListResp{
		Total: uint64(total),
		Data:  make([]*job.RelayChainInfo, 0, len(list)),
	}
	for _, v := range list {
		resp.Data = append(resp.Data, toRelayChainInfo(v))
	}

	return resp, nil
}

type GetRelayChainByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRelayChainByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRelayChainByIdLogic {
	return &GetRelayChainByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRelayChainByIdLogic) GetRelayChainById(in *job.RelayChainIDReq) (*job.RelayChainInfo, error) {
	result, err := l.svcCtx.DB.RelayChain.Get(l.ctx, in.Id)
	if err != nil {
		return nil, dberrorhandler.DefaultEntError(l.Logger, err, in)
	}

	return toRelayChainInfo(result), nil
}
