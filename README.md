# zhuzhao-utils

**简体中文 | [English](README.en.md)**

**zhuzhao 产品族共用的 Go 基础机制，按包引用。**

签名、令牌、错误响应、日志和数据库连接等重复代码集中维护，业务数据和授权决策留在各自服务。可以只引用需要的包，无需启动 zhuzhao。

产品族：[zhuzhao](https://github.com/tracerbiubiubiu/zhuzhao) · [zhuzhao-ui](https://github.com/tracerbiubiubiu/zhuzhao-ui) · [activelist](https://github.com/tracerbiubiubiu/activelist) · [taskrunner](https://github.com/tracerbiubiubiu/taskrunner) · **zhuzhao-utils**

## 包与用途

- **`aksk`**：内部 HMAC-SHA256 签名／验签、HTTP Transport 和 Gin 中间件；`aksk/compat` 提供独立的存量协议适配。
- **`jwt`**：HS256 access／refresh token，令牌类型区分与密码纪元字段。
- **`crypto`**：bcrypt 密码哈希与校验。
- **`errcode`、`response`**：错误码机制与 Gin 响应。
- **`logger`**：slog 与日志轮转。
- **`postgres`、`redis`**：pgxpool／go-redis 构造器，以及 Redis 登录失败锁定器。
- **`jsonutil`、`validate`**：int64 数组 JSON 编解码、标识符等纯校验。

准确符号与版本边界见 [公共包/API](docs/api/README.md)及各包源码。不同包的实际依赖由 Go 模块和导入图决定。

## 一个可运行的签名往返示例

在新的空目录执行。本例固定使用发布记录中的 **v0.4.3**，该 tag 的 `go.mod` 要求 Go **1.26.0**；当前开发分支的最低 Go **1.26.11**、推荐工具链 **1.27.4** 是另一条维护基线，见 [开发指南](docs/guides/development.md)。

```sh
go mod init example.com/aksk-demo
go get github.com/tracerbiubiubiu/zhuzhao-utils/aksk@v0.4.3
```

将以下完整程序保存为 `main.go`：

```go
package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/tracerbiubiubiu/zhuzhao-utils/aksk"
)

func main() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	body := []byte(`{"action":"demo"}`)
	req, err := http.NewRequest(http.MethodPost,
		"https://example.invalid/tasks", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	aksk.Sign(req, body, aksk.SignOptions{
		AK: "demo-client", SK: key,
		Operator: "demo-user", RequestID: "readme-demo",
	})
	verifier := &aksk.Verifier{Keys: map[string][]byte{"demo-client": key}}
	if err := verifier.Verify(req, body); err != nil {
		panic(err)
	}
	fmt.Println("signature verified")
}
```

```sh
go run .
```

预期输出 `signature verified`。密钥仅在本次进程内生成，不会打印；程序不发送 HTTP 请求、不需要数据库或 Redis。首次安装依赖需要模块网络或已准备的缓存。实际客户端／服务端接线见 [用法](docs/guides/usage.md)。

## 版本与安全边界

消费方应固定 tag 或不可变 pseudo-version，并保留 `go.sum`。v0.4.3 是本示例的明确基线，不是“当前最新版本”的承诺。后续 Reader／Streaming 和审计修复需核包含该实现的精确版本；不能用本地 HEAD 的符号推定旧 tag 可用。详见 [发布记录](docs/ops/releases.md)与 [版本升级](docs/ops/version-upgrade.md)。

内部签名覆盖 method、path、body、时间戳、request ID 与 operator；**query 不入签，时间窗也不是单次 nonce**。服务身份、操作者归因、业务授权及效果幂等由不同层承接，使用方法见 [AK/SK 契约](docs/api/aksk.md)。JWT 密码纪元的存储、递增和当前值比对同样由消费方负责。

本库只收公共机制。业务表、资源权限、模型 SDK、能力目录和任务状态不放进 utils；规划中的 [平台公共机制合同](docs/api/platform-contracts.md)不表示新 SDK 或新 wire 已发布。

## 文档与贡献

- [文档导航](docs/README.md) · [用法](docs/guides/usage.md) · [公共包/API](docs/api/README.md)。
- [开发指南](docs/guides/development.md) · [协作协议](AGENTS.md)：先写合同／正反例，再改实现，核实际消费者。

```sh
make hooks
make lint
make test
make build
```

本仓的 `make lint` 包含 vet、格式检查和测试。公共 API 变更请说明版本与消费者影响，欢迎通过 [Issues](https://github.com/tracerbiubiubiu/zhuzhao-utils/issues)或聚焦的 PR 参与。

## 许可

[MIT](LICENSE)。
