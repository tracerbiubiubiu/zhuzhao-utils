# zhuzhao-utils

后台服务通用工具库，从 [zhuzhao](https://github.com/tracerbiubiubiu/zhuzhao) 的 `internal/pkg` 抽取而来。

## 包一览

| 包 | 用途 |
|---|---|
| `crypto` | bcrypt 密码哈希 / 校验，含防时序侧信道的恒定失败比对 |
| `errcode` | 业务错误码框架 + 通用错误码（10000–10999）；业务码用 `errcode.New` 自行定义 |
| `jsonutil` | `Int64Slice`：JSON 数组元素兼容 string / number |
| `validate` | ltree 标签、业务标识符的字符合法性校验 |
| `response` | gin 统一响应结构（依赖 `errcode`） |
| `logger` | slog + lumberjack 落地日志 |
| `postgres` | pgxpool 连接池构建器（describe 缓存、statement_timeout 等调优内建） |
| `redis` | go-redis 客户端构建器 + 登录失败锁定器（Lua 原子脚本） |
| `jwt` | HS256 双令牌（access / refresh）签发与解析，typ 声明防令牌混淆 |
| `aksk` | ✅ **已实现（2026-09-04）**：服务间 AK/SK HMAC-SHA256 签名/验签——`Sign` 出站签名 + `Verifier` 验签 + `GinMiddleware` 服务端中间件 + `Transport` 出站 RoundTripper 自动签名；canonical = METHOD\nPATH\nsha256(body)\nTS\nX-Request-ID\nX-Operator（可选头入签名覆盖即不可篡改；**PATH 空路径归一化为 `/`**——自实现签名侧必须同样处理，否则根路径验签必失配）；常量时间比较 + ±5min 时间窗防重放。**边界**：URL query 不入签（API 约定 POST 业务参数放 body、GET query 不受完整性保护）；中间件读体上限默认 8MB（`Verifier.MaxBodyBytes`），缺头先拒不读体，读体失败/超限与验签失败分开报错（400/413 vs 401）；`Transport` 在请求克隆上签名，不修改调用方 request。设计 SSOT = zhuzhao `docs/phase3/16-external-integration.md` §9；首个消费者 = taskrunner C2/C9、activelist M-A6、zhuzhao E-②/E-④/批次 B |

各包相互独立（`response` 依赖 `errcode` 除外），按需引用，不会带入无关依赖。

## 使用

要求 **zhuzhao-utils ≥ v0.2.0**（`aksk` 自 v0.2.0 起提供，v0.1.0 不含）：

```sh
go get github.com/tracerbiubiubiu/zhuzhao-utils@v0.2.0
```

```go
import (
    "github.com/tracerbiubiubiu/zhuzhao-utils/crypto"
    "github.com/tracerbiubiubiu/zhuzhao-utils/errcode"
)
```

### aksk 最小用法

```go
// 出站签名（客户端侧）：Transport 在请求克隆上签名，不改调用方 request
client := &http.Client{Transport: &aksk.Transport{
    AK: "zhuzhao", SK: []byte(os.Getenv("ZHUZHAO_SK")),
    Base: http.DefaultTransport,
}}
// 服务端验签（被调侧）：密钥环 = 预期调用方 AK→SK（空 SK 条目视同未知凭据）。
// 验签通过后自动写入 gin context 归因键 caller（调用方 AK）/ operator（X-Operator，
// 缺省 "system"）；失败现场经 Verifier.Logger 落日志。
v := &aksk.Verifier{Keys: map[string][]byte{"zhuzhao": []byte(sk)}, Logger: logger}
r.Use(aksk.GinMiddleware(v, response.AKSKFail()))
```

要点：canonical 覆盖 METHOD/PATH（空路径归一化 `/`）/body 哈希/TS/`X-Request-ID`/`X-Operator`；URL query 不入签（安全参数放 body）；TS ±5min 防重放；读体上限默认 8MB（`Verifier.MaxBodyBytes` 可调）。

> 版本注记：`Verifier.Logger` 与 `response.AKSKFail()` 自 **v0.4.0** 起；归因键常量 `aksk.ContextKeyCaller`/`aksk.ContextKeyOperator`（消费方取 gin context 归因时引用，禁裸字符串）及 AKSKFail 格式提示修正自 **v0.4.1** 起，**以发布 tag 为准 pin**。

各基建包（`logger` / `postgres` / `redis` / `jwt`）自带 `Config` 结构体，零值字段取安全默认值，不绑定任何配置框架；由调用方把自己（viper / yaml / env）的配置映射进来。

## License

MIT

> **`aksk` 密钥模型**：按调用方发 SK（zhuzhao / taskrunner / activelist 各一把，env 注入各自容器）；被调方持有预期调用方的 SK 小密钥环验签（**SK 条目不得为空——空条目视同未知凭据，验签恒拒 fail-closed**）。内部静态密钥、无管理面；**外部 M2M** AK/SK（DB 存储/哈希/签发吊销管理面）仍 🚦 随外部调用方出现，届时算法层直接复用本包（zhuzhao phase3/09 分层注记）。
