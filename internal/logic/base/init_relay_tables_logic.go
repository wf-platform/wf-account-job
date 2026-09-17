package base

import (
	"context"

	entschema "entgo.io/ent/dialect/sql/schema"
	"github.com/suyuan32/simple-admin-common/enum/errorcode"
	"github.com/suyuan32/simple-admin-common/i18n"
	"github.com/suyuan32/simple-admin-common/msg/logmsg"
	"github.com/zeromicro/go-zero/core/errorx"
	"github.com/zeromicro/go-zero/core/logx"

	"wf-account-job/ent/migrate"
	"wf-account-job/internal/svc"
	"wf-account-job/types/job"
)

type InitRelayTablesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInitRelayTablesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InitRelayTablesLogic {
	return &InitRelayTablesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InitRelayTablesLogic) InitRelayTables(in *job.Empty) (*job.BaseResp, error) {
	tables := []*entschema.Table{
		migrate.RelayChainTable,
		migrate.RelayTokenTable,
	}

	if err := migrate.Create(l.ctx, l.svcCtx.DB.Schema, tables, migrate.WithForeignKeys(false)); err != nil {
		logx.Errorw(logmsg.DatabaseError, logx.Field("detail", err.Error()))
		return nil, errorx.NewCodeError(errorcode.Internal, err.Error())
	}

	return &job.BaseResp{
		Msg: i18n.Success,
	}, nil
}
