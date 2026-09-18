# wf-account-job

`wf-account-job` 是一个基于 Go、go-zero 和 Asynq 的 RPC 定时任务服务，为
WF Admin 提供任务管理、任务日志和后台任务调度能力。

项目当前包含以下能力：

- 通过 gRPC 管理定时任务
- 使用 Asynq 执行异步任务和周期任务
- 记录和查询任务日志
- 同步、查询 Relay Chain 和 Relay Token 信息
- 提供可供 API 项目直接引用的 `jobclient` 客户端包
- 提供 Linux amd64 二进制和 GHCR Docker 镜像构建入口

## 环境要求

- Go 1.26.0 或更高版本
- PostgreSQL
- Redis
- Docker（可选，用于容器化运行）

项目使用公有 Go module，模块路径为：

```text
github.com/wf-platform/wf-account-job
```

## 快速开始

### 1. 准备配置文件

在项目根目录执行：

```bash
cp .env.example .env
cp .env.job.example .env.job
```

根据本地环境修改 `.env` 和 `.env.job`，至少确认数据库、Redis 和服务监听地址配置正确。
配置示例文件不会包含真实密码，生产环境请使用环境变量或安全的配置注入方式提供敏感信息。

### 2. 启动依赖服务

默认配置使用：

- PostgreSQL：`localhost:5432`
- Redis：`127.0.0.1:6379`
- 数据库名：`wf`
- RPC 监听地址：`0.0.0.0:9105`

启动服务前，请先创建数据库，并确保 Job 服务可以访问 PostgreSQL 和 Redis。

### 3. 启动 Job RPC 服务

默认使用 `dev` 配置：

```bash
go run .
```

也可以通过 `APP_ENV` 选择环境配置：

```bash
APP_ENV=test go run .
APP_ENV=pre go run .
APP_ENV=prod go run .
```

配置文件按以下方式加载：

1. 读取根目录 `.env`
2. 读取服务配置 `.env.job`
3. 根据 `APP_ENV` 合并 `etc/job.yaml` 和对应环境文件，例如 `etc/job-dev.yaml`
4. 已存在于进程环境中的变量优先级最高

也可以通过 `-f` 直接指定完整配置文件：

```bash
go run . -f etc/job-prod.yaml
```

开发和测试环境会注册 gRPC reflection，方便使用 grpcurl 等工具检查服务。

## 初始化数据库

数据库表由服务提供 RPC 初始化接口：

- `initDatabase`
- `initRelayTables`

API 项目接入后，可以通过 `jobclient.Job` 调用这两个接口。初始化操作应在部署流程中显式执行，
不要在每次服务启动时重复执行。

## API 项目接入

### 安装依赖

在 API 项目目录执行：

```bash
go get github.com/wf-platform/wf-account-job@v0.2.0
go mod tidy
```

使用其他已发布版本时，将 `v0.2.0` 替换为对应版本号。

### 创建 RPC 客户端

`jobclient` 封装了 Job RPC 服务的客户端和请求响应类型：

```go
package jobrpc

import (
	"github.com/wf-platform/wf-account-job/jobclient"
	"github.com/zeromicro/go-zero/zrpc"
)

func NewClient(endpoint string) (jobclient.Job, error) {
	client, err := zrpc.NewClient(zrpc.RpcClientConf{
		Endpoints: []string{endpoint},
	})
	if err != nil {
		return nil, err
	}

	return jobclient.NewJob(client), nil
}
```

建议在 API 项目的 ServiceContext 初始化时创建并复用客户端。例如：

```go
type Config struct {
	JobRpc zrpc.RpcClientConf
}
```

对应 YAML 配置：

```yaml
JobRpc:
  Endpoints:
    - 127.0.0.1:9105
```

`127.0.0.1:9105` 只适用于 API 和 Job 服务运行在同一主机或同一网络命名空间的情况。
容器或跨主机部署时，应替换为 API 可以访问的 Job 服务地址。

不要从 API 项目引用本仓库的 `internal` 包；公共接入面是 `jobclient`。

### 可用 RPC 分组

`jobclient.Job` 当前提供以下接口分组：

- 数据库初始化：`InitDatabase`、`InitRelayTables`
- 任务管理：创建、更新、分页查询、按 ID 查询、删除
- 任务日志：创建、更新、分页查询、按 ID 查询、删除
- Relay Chain：列表查询、按 ID 查询
- Relay Token：列表查询、按 Chain 和 Token ID 查询
- 面向客户端的 Relay Chain 和 Relay Token 查询

## 配置说明

### 通用配置

通用配置位于 `.env`：

```dotenv
APP_ENV=dev

DATABASE_TYPE=postgres
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USERNAME=go_user
DATABASE_PASSWORD=change-me

REDIS_HOST=127.0.0.1:6379
REDIS_DB=0
REDIS_MODE=single
REDIS_PASSWORD=change-me
```

### Job 配置

服务配置位于 `.env.job`，常用变量包括：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `JOB_LISTEN_ON` | `0.0.0.0:9105` | gRPC 监听地址 |
| `JOB_DATABASE_DB_NAME` | `wf` | Job 使用的数据库 |
| `JOB_ASYNQ_ENABLE` | `true` | 是否启用 Asynq |
| `JOB_ASYNQ_CONCURRENCY` | `20` | Asynq 并发数 |
| `JOB_ASYNQ_SYNC_INTERVAL` | `10` | 周期任务同步间隔 |
| `JOB_ENABLE_SCHEDULED_TASK` | `false` | 是否启用内置示例调度任务 |
| `JOB_ENABLE_DP_TASK` | `true` | 是否启用动态周期任务 |
| `JOB_RELAY_CHAINS_URL` | `https://api.relay.link/chains` | Relay Chain 数据地址 |
| `JOB_PROMETHEUS_PORT` | `4005` | Prometheus 指标端口 |

环境覆盖文件位于 `etc/`：

- `etc/job-dev.yaml`
- `etc/job-test.yaml`
- `etc/job-pre.yaml`
- `etc/job-prod.yaml`

## Docker

### 本地构建

先构建 Linux 二进制，再构建镜像：

```bash
make build-linux
docker build -t wf-account-job-rpc:local .
```

运行容器时，需要将配置和数据依赖地址配置为容器可访问的地址：

```bash
docker run --rm \
  --env-file .env \
  --env-file .env.job \
  -p 9105:9105 \
  wf-account-job-rpc:local
```

### GHCR 镜像

GitHub Actions 会构建并推送以下镜像：

```text
ghcr.io/wf-platform/wf-account-job-rpc:latest
```

拉取镜像：

```bash
docker pull ghcr.io/wf-platform/wf-account-job-rpc:latest
```

镜像默认启动 `job_rpc`，监听端口为 `9105`。数据库和 Redis 仍需要单独部署，
容器不会自动创建外部依赖服务。

## 常用命令

```bash
# 运行测试
make test

# 格式化 Go 代码
make fmt

# 执行 lint
make lint

# 构建 Linux amd64 二进制
make build-linux

# 构建 macOS amd64 二进制
make build-mac

# 构建 Windows amd64 二进制
make build-win

# 生成 RPC 代码
make gen-rpc

# 生成 Ent 代码
make gen-ent
```

`make build-linux` 生成 `job_rpc`，`make build-mac` 生成 `job_rpc`，
`make build-win` 生成 `job_rpc.exe`。

## 代码生成

修改 `job.proto` 后执行：

```bash
make gen-rpc
```

修改 `ent/schema` 下的数据库 schema 后执行：

```bash
make gen-ent
```

生成代码已提交到仓库，普通使用者不需要重新安装代码生成工具。修改协议或 schema 后，
请同时检查生成文件和 API 项目的兼容性。

## 发布版本

本仓库使用 Git tag 标记版本。发布新版本前，先确认代码已经合并到 `master`，然后执行：

```bash
git tag v0.3.0
git push origin v0.3.0
```

版本号建议遵循语义化版本格式，例如：

- `v0.3.0`
- `v0.3.1`
- `v0.4.0-beta.1`

发布前请确认：

- `.env.example` 和 `.env.job.example` 没有真实密码或 token
- 版本号与 `go.mod` 模块路径保持一致
- RPC 或数据库 schema 变更已经重新生成代码
- Docker 镜像可以正常启动

## 项目结构

```text
.
├── job.go                 # RPC 服务入口
├── job.proto              # RPC 接口定义
├── jobclient/             # API 项目使用的客户端包
├── types/job/             # Protobuf 生成代码
├── internal/config/       # 服务配置结构
├── internal/server/       # RPC 服务实现
├── internal/logic/        # 业务逻辑
├── internal/mqs/          # Asynq 任务和处理器
├── ent/schema/            # Ent 数据库 schema
├── etc/                   # 环境配置覆盖文件
├── Dockerfile             # Docker 镜像定义
└── Makefile               # 构建和代码生成命令
```

## License

本项目源码遵循仓库中声明的许可证。使用前请以仓库实际许可证文件和版权声明为准。
