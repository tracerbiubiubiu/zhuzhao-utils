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

各包相互独立（`response` 依赖 `errcode` 除外），按需引用，不会带入无关依赖。

## 使用

```go
import (
    "github.com/tracerbiubiubiu/zhuzhao-utils/crypto"
    "github.com/tracerbiubiubiu/zhuzhao-utils/errcode"
)
```

各基建包（`logger` / `postgres` / `redis` / `jwt`）自带 `Config` 结构体，零值字段取安全默认值，不绑定任何配置框架；由调用方把自己（viper / yaml / env）的配置映射进来。

## License

MIT
