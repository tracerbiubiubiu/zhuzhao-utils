# zhuzhao-utils

后台服务通用工具库，从 [zhuzhao](https://github.com/tracerbiubiubiu/zhuzhao) 的 `internal/pkg` 抽取而来。

## 生态角色

zhuzhao 生态（[zhuzhao](https://github.com/tracerbiubiubiu/zhuzhao) IAM+网关 / [taskrunner](https://github.com/tracerbiubiubiu/taskrunner) 任务总线 / [activelist](https://github.com/tracerbiubiubiu/activelist) 动态数据）的公共底座：语义化版本发版，消费方 pin 去 replace（临时 replace 仅限联调窗口）。抽取边界 = 只抽无数据依赖的纯工具；工程公约 SSOT 见 zhuzhao 仓 `docs/standards.md`（§5 公共包）。

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
| `aksk` | ✅ **已实现（2026-09-04）**：服务间 AK/SK HMAC-SHA256 签名/验签——`Sign` 出站签名 + `Verifier` 验签 + `GinMiddleware` 服务端中间件 + `Transport` 出站 RoundTripper 自动签名；canonical = METHOD\nPATH\nsha256(body)\nTS\nX-Request-ID\nX-Operator（可选头入签名覆盖即不可篡改；**PATH 空路径归一化为 `/`**——自实现签名侧必须同样处理，否则根路径验签必失配）；常量时间比较 + ±5min 时间窗防重放。**边界**：URL query 不入签（API 约定 POST 业务参数放 body、GET query 不受完整性保护）；中间件读体上限默认 8MiB（`Verifier.MaxBodyBytes`），缺头先拒不读体，读体失败/超限与验签失败分开报错（400/413 vs 401）；`Transport` 在请求克隆上签名，不修改调用方 request。设计 SSOT = zhuzhao `docs/phase3/16-external-integration.md` §9；首个消费者 = taskrunner C2/C9、activelist M-A6、zhuzhao E-②/E-④/批次 B |
| `aksk/compat` | ✅ **已实现（2026-10-13）**：存量框架 `x-auth-*` 协议兼容层（**独立重实现，不引原库**，与 `aksk` 包零耦合）——`Sign`/`Verifier`/`GinMiddleware`/`Transport`（出站 RoundTripper 自动签名，挂 `http.Client`，调用存量接口免手签），签名/验签/中间件/出站形态与 `aksk` 对齐；线协议以包内测试冻结向量为准（向量由存量实现现场生成并经其服务端校验器反向验证 + 双向交叉验证四方向 PASS）。我方端补强：±5min 时间窗（存量不校验）/恒时比较/读体上限 8MiB/空 SK fail-closed/caller=AK 归因（**存量协议无 operator 头，归因仅 caller**，与内部 caller+operator 双键口径不同）；默认响应码值对齐内部（读体 10001/鉴权 10002），分档文案经 `onFail` 传自定义实现。**已知让渡**：签名不覆盖 method/path（跨端点重放由部署纪律承接——**兼容中间件只挂专用路由组，且组内实质单端点或组内端点幂等/无副作用**（路由组只隔离组外，防不了组内互相重放），standards §3.11 登记豁免）、防重放仅时间窗无 nonce（窗口内由业务幂等承接）；`Verifier.SkipBody`（对端中间件 Config.SkipBody 同名同语义）显式开启后接受 body 不入签的存量 skipBody 形态——**默认关闭**（默认仅真空体接受空签名），抓包确认存量流量确有此形态后再开。出处与翻案决策 = zhuzhao `docs/acknowledgments.md`、phase1/02-auth §存量迁移追记 |

各包相互独立（`response` 依赖 `errcode` 与 `aksk`[AKSKFail 分档]；`aksk/gin.go` 与 `aksk/compat/gin.go` 的默认响应引用 `errcode` 码值常量——均为同模块纯常量/哨兵包，无数据面耦合），按需引用，不会带入无关依赖。

## 使用

要求 **zhuzhao-utils ≥ v0.4.2**（本 README 示例口径；`aksk` 自 v0.2.0 起提供，v0.1.0 不含；`aksk/compat` 自 v0.4.2 起提供）：

```sh
go get github.com/tracerbiubiubiu/zhuzhao-utils@v0.4.2
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

要点：canonical 覆盖 METHOD/PATH（空路径归一化 `/`）/body 哈希/TS/`X-Request-ID`/`X-Operator`；URL query 不入签（安全参数放 body）；TS ±5min 防重放；读体上限默认 8MiB（`Verifier.MaxBodyBytes` 可调）。

大文件入口可单独使用 `aksk.GinStreamingMiddleware(verifier, tempDir, response.AKSKFail())`：按 `MaxBodyBytes` 有界流式验签，同时暂存私有 0600 文件；认证成功后 rewind 才进入 handler，归因与普通中间件一致。普通 CRUD 继续使用原内存中间件。`tempDir=""` 使用系统临时目录，部署须准备与请求并发/大小匹配的磁盘容量和读取超时；正常返回、业务失败、panic 展开均清理，SIGKILL/进程崩溃残留由系统临时目录清理策略承接。请求读失败400、超限413、鉴权401、服务器暂存故障503，不回显内部路径。

非 Gin 调用方可用 `Verifier.VerifyReader(request, reader)` 流式校验；它不还原或关闭 reader，不保存 body，最多探读上限外一字节，先校验头再读体、读后重验时间窗。业务写必须在验签成功后执行。取消会在每次 Read 前后检查 context，底层阻塞 reader 的打断/关闭仍由调用方负责。新接口目前位于独立开发分支，消费者以包含它的不可变版本 pin，不代表已发布新 tag。

> 版本注记：`Verifier.Logger` 与 `response.AKSKFail()` 自 **v0.4.0** 起；归因键常量 `aksk.ContextKeyCaller`/`aksk.ContextKeyOperator`（消费方取 gin context 归因时引用，禁裸字符串）及 AKSKFail 格式提示修正自 **v0.4.1** 起，**以发布 tag 为准 pin**。

各基建包（`logger` / `postgres` / `redis` / `jwt`）自带 `Config` 结构体，零值字段取安全默认值，不绑定任何配置框架；由调用方把自己（viper / yaml / env）的配置映射进来。

## 开发

```sh
make lint    # vet + gofmt 漂移检查 + test
make test    # go test ./...
make build   # go build ./...
```

CI（GitHub Actions）：push(main / tag `v*`) + PR → vet + gofmt + test -race。

## 版本历史

| 版本 | 增量 |
|---|---|
| v0.4.3 | `jwt.AccessClaims.Pwe` + `GenerateAccessTokenWithEpoch`（B02-F01 候选；旧签发入口保持纪元 0）；`aksk/gin.go`·`aksk/compat/gin.go` 默认响应码值改引 `errcode` 常量（wire 形态零变化）；**哨兵/构造错误消息补包名前缀（jwt/redis/postgres/jsonutil——⚠ 消费方可感变更**：按消息文本做的日志解析或断言需同步，`errors.Is` 判定不受影响）；crypto/errcode/logger 三包补测 + jwt AT 过期哨兵用例；README 依赖边界声明修正。驱动=2026-10-15 清欠批（zhuzhao review/11 §8 BK-25） |
| v0.4.2 | `aksk/compat` 包：存量框架 `x-auth-*` 协议兼容层（签名/验签/中间件 + 出站自动签名 `Transport`，独立重实现；挂载待目标服务）；`jsonutil.Int64Slice` 补 `MarshalJSON`（发送侧 ID 数组一律字符串，配 nil→`[]` 数组恒不 null）；jwt `ParseRefreshToken` 过期错误包装 `ErrTokenExpired` 哨兵（errors.Is 可判）；postgres 移除 `MaxIdleConns` 口径误映射 |
| v0.4.1 | 归因键常量 `ContextKeyCaller/ContextKeyOperator` + AKSKFail 文案修正（头字段名 Signature=） |
| v0.4.0 | 验签统一批：`Verifier.Logger` 验签失败日志、`GinMiddleware` 归因键收编（caller/operator 写入 gin context）、`response.AKSKFail()` 标准 onFail |
| v0.3.0 | `RefreshClaims.Pwe`（密码纪元防令牌存活越权） |
| v0.2.0 | `aksk` 包：服务间 AK/SK HMAC 签名/验签（Sign/Verifier/GinMiddleware/Transport） |

## aksk 密钥模型（设计注记）

> **`aksk` 密钥模型**：按调用方发 SK（zhuzhao / taskrunner / activelist 各一把，env 注入各自容器）；被调方持有预期调用方的 SK 小密钥环验签（**SK 条目不得为空——空条目视同未知凭据，验签恒拒 fail-closed**）。内部静态密钥、无管理面；**外部 M2M** AK/SK（DB 存储/哈希/签发吊销管理面）仍 🚦 随外部调用方出现，届时算法层直接复用本包（zhuzhao phase3/09 分层注记）。

## AT 密码纪元接入（v0.4.3）

v0.4.3（关联审计 `ECO-20261104-B02-F01`）：`jwt.AccessClaims.Pwe` 与
`Manager.GenerateAccessTokenWithEpoch` 为 AT 增加签发纪元；旧签发入口保留纪元 0。
纪元存储、递增及请求时比对属于消费方职责，本包仅签发/解析。
消费方需 pin v0.4.3 或后续包含该 API 的正式版本；本地工作区联调不能替代版本 pin 验证。

## 提交规范

遵 zhuzhao `docs/standards.md` §12-9（全生态公约）：`type(scope): ` 中文主题行 **≤50 字** + 评审三节正文（为什么改 / 改动摘要 / 验证证据）；本仓机械门禁（commit-lint / .githooks / make hooks）随兄弟仓对齐批落地（2026-11-02 四仓审计登记）。

## License

MIT
