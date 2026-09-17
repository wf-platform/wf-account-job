package config

import (
	"github.com/suyuan32/simple-admin-common/config"
	"github.com/suyuan32/simple-admin-common/plugins/mq/asynq"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DatabaseConf config.DatabaseConf
	RedisConf    config.RedisConf
	AsynqConf    asynq.AsynqConf
	TaskConf     TaskConf
	RelayConf    RelayConf
}

type TaskConf struct {
	EnableScheduledTask bool `json:",default=true"`
	EnableDPTask        bool `json:",default=true"`
}

type RelayConf struct {
	ChainsURL string `json:",default=https://api.relay.link/chains"`
}
