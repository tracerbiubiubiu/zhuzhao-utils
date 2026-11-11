# 公共包与契约

本库只收无业务数据依赖的通用工具，不收资源库表、权限策略事实或业务handler。包源码及Go doc仍在各包目录，module path不变；本页为索引，不代替具体函数签名。

## 按用途找包

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
| `aksk` | 服务间HMAC-SHA256签名/验签：`Sign`、`Verifier`、Gin中间件和出站`Transport`；签名字段、8MiB默认限体、时间窗与Reader/Streaming边界见[内部协议](aksk.md#内部协议)。 |
| `aksk/compat` | 存量`x-auth-*`协议的独立兼容实现；不覆盖method/path、无operator、SkipBody默认关闭，迁移接线与限制见[存量协议](aksk.md#存量compat协议)。 |

各包相互独立（`response` 依赖 `errcode` 与 `aksk`[AKSKFail 分档]；`aksk/gin.go` 与 `aksk/compat/gin.go` 的默认响应引用 `errcode` 码值常量——均为同模块纯常量/哨兵包，无数据面耦合），按需引用，不会带入无关依赖。

## 接入与版本

使用步骤见[接入指南](../guides/usage.md)，签名、失败响应及重放边界集中维护在[AK/SK契约](aksk.md)。后续 R07/P7 的调用/身份、纯机制边界与版本兼容见[平台公共机制合同](platform-contracts.md)。内部协议的设计出处是主仓[服务集成设计§9](../../../zhuzhao/docs/phase3/16-external-integration.md)，compat的来源与退役决定见主仓[致谢](../../../zhuzhao/docs/acknowledgments.md)及[认证设计的存量迁移追记](../../../zhuzhao/docs/phase1/02-auth.md)。

包存在不代表所有发布tag都包含它；版本/tag与Reader/Streaming等未发布候选见[发布记录](../ops/releases.md)。
