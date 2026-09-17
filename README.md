# wf-account-job


WF Account Job 是一个 wf admin 的在线定时任务扩展模块。

目前支持： 基于 asynq 的定时任务

---

WF Account job is a rpc module for simple admin to do online job schedule.

Support: asynq schedule task

## API 项目接入（私有 Go 模块）

模块路径为 `github.com/wf-platform/wf-account-job`，需要 Go 1.26.0 或更高版本。
API 项目引用 `jobclient` 调用独立部署的 Job RPC 服务；请求和响应类型也由
`jobclient` 导出。不要引用本项目的 `internal` 包。

### 配置仓库读取权限

同属一个 GitHub 组织不会自动获得私有仓库的读取权限。开发账号需要具备该仓库的
读取权限，并将本机 SSH 公钥添加到 GitHub；组织启用 SSO 时还需完成相应授权。

在开发电脑配置私有模块和组织仓库的 SSH 下载方式：

```bash
go env -w GOPRIVATE='github.com/wf-platform/*'
git config --global url."git@github.com:wf-platform/".insteadOf "https://github.com/wf-platform/"
git ls-remote git@github.com:wf-platform/wf-account-job.git
```

如果已有其他 `GOPRIVATE` 配置，将 `github.com/wf-platform/*` 追加到原值，使用逗号分隔。
`GOPRIVATE` 默认让匹配的模块绕过公共模块代理和校验服务，它本身不提供仓库访问凭据。

### 安装依赖

先将模块路径迁移后的代码提交并推送，再发布版本标签。以下以已经发布的 `v0.1.0`
为例，在 API 项目目录执行；也可以将版本替换为包含迁移代码的提交哈希：

```bash
go get github.com/wf-platform/wf-account-job@v0.1.0
```

Go 不会继承依赖模块中的 `replace`。API 项目如需与本服务使用相同的框架实现，
应在自身的 `go.mod` 中加入以下配置；已有替换时先确认版本兼容性：

```go
replace github.com/zeromicro/go-zero v1.10.2 => github.com/suyuan32/simple-admin-tools v1.10.2
```

该替换只匹配 `go-zero v1.10.2`，API 如果使用其他版本，需要自行统一版本及对应的替换。
导入客户端并配置好依赖后，在 API 项目执行 `go mod tidy`。

### 创建 RPC 客户端

以下为可放入 API 项目的客户端初始化示例，RPC 地址通过参数传入：

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

建议在 API 的 ServiceContext 初始化时创建并复用客户端。如果 API 已使用 go-zero
配置管理，也可以在配置结构体中添加 `JobRpc zrpc.RpcClientConf`，在 YAML 中配置：

```yaml
JobRpc:
  Endpoints:
    - 127.0.0.1:9105
```

然后将 `c.JobRpc` 传给 `zrpc.NewClient`。`127.0.0.1:9105` 仅适用于 API 和 Job
运行在同一主机网络的情况；容器或跨主机部署时应改成 API 可访问的 Job 服务地址。
引入 Go 依赖不会启动 Job 服务，需要单独部署服务并配置数据库、Redis 和 RPC 监听地址。

### CI 构建

API 的 CI 构建环境同样需要 `GOPRIVATE=github.com/wf-platform/*` 和仓库读取凭据。
GitHub Actions 默认的 `GITHUB_TOKEN` 通常不能读取另一个私有仓库，即使属于同一组织。

可以选择目标仓库的只读 Deploy Key（SSH），或具备目标仓库 `Contents: Read` 权限的
GitHub App token / fine-grained PAT（HTTPS）。将凭据保存在 CI Secrets 中，在执行
`go mod download` 或构建前配置认证，不要将凭据写入源码、`go.mod` 或 Docker 镜像。
SSH 方式需配置密钥、已验证的 GitHub 主机公钥和上述 URL 重写；HTTPS 方式使用安全的
Git 凭据配置，并避免被 SSH URL 重写覆盖。

### 代码生成

修改 RPC 定义后继续使用 `make gen-rpc`，修改 Ent schema 后使用 `make gen-ent`。
生成器从当前 `go.mod` 读取模块路径。Proto 中的 `go_package = "./job"` 配合现有
`--go_out=./types` 保持生成文件位于 `types/job`，无需为了远程引用改为完整仓库路径。
