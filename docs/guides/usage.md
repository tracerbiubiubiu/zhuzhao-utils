# 接入与最小用法

在消费方Go模块根目录执行go get。按需要选择已发布tag；本页的v0.4.2基础示例不包含后来才提供的Reader/Streaming/AT epoch全部能力。先看 [版本边界](../ops/releases.md)，再pin所需版本。



要求 **zhuzhao-utils ≥ v0.4.2**（本页基础示例口径；`aksk` 自 v0.2.0 起提供，v0.1.0 不含；`aksk/compat` 自 v0.4.2 起提供）：

```sh
go get github.com/tracerbiubiubiu/zhuzhao-utils@v0.4.2
```

## aksk 最小用法

以下是放在应用初始化函数内的**接线片段**，不是独立的 `main` 程序。按需要采用客户端或服务端部分；调用方仍需注册业务路由、设置监听地址或实际发起请求。先从环境或密钥挂载取得非空SK，本例使用环境变量 `ZHUZHAO_SK`；示例AK `zhuzhao` 须与被调方密钥环中的键一致。

下面的导入对应实际使用的符号：`http` 创建客户端，`gin` 创建服务端路由，`slog.Default()` 提供验签诊断日志，`aksk` 与 `response` 提供本库的协议接线。

```go
import (
    "log/slog"
    "net/http"
    "os"

    "github.com/gin-gonic/gin"
    "github.com/tracerbiubiubiu/zhuzhao-utils/aksk"
    "github.com/tracerbiubiubiu/zhuzhao-utils/response"
)
```

```go
sk := []byte(os.Getenv("ZHUZHAO_SK"))
if len(sk) == 0 {
    panic("初始化前须注入非空 ZHUZHAO_SK")
}
// 出站签名（客户端侧）：Transport 在请求克隆上签名，不改调用方 request
client := &http.Client{Transport: &aksk.Transport{
    AK: "zhuzhao", SK: sk,
    Base: http.DefaultTransport,
}}
// 将 client 交给应用实际的出站请求代码；超时按消费方要求配置。

// 服务端验签（被调侧）：密钥环 = 预期调用方 AK→SK（空 SK 条目视同未知凭据）。
// 验签通过后自动写入 gin context 归因键 caller（调用方 AK）/ operator（X-Operator，
// 缺省 "system"）；失败现场经 Verifier.Logger 落日志。
r := gin.New()
v := &aksk.Verifier{Keys: map[string][]byte{"zhuzhao": sk}, Logger: slog.Default()}
r.Use(aksk.GinMiddleware(v, response.AKSKFail()))
// 在 r 上继续注册业务 handler；验签成功后才进入这些 handler。
```

要点：canonical 覆盖 METHOD/PATH（空路径归一化 `/`）/body 哈希/TS/`X-Request-ID`/`X-Operator`；URL query 不入签（安全参数放 body）。TS默认接受±5min时间窗，只限制过期请求；协议没有nonce（一次性请求标记）的单次消费，窗口内重放仍需业务幂等承接。读体上限默认8MiB（`Verifier.MaxBodyBytes`可调）；完整安全边界见[内部协议](../api/aksk.md#内部协议)。

大文件入口可单独使用 `aksk.GinStreamingMiddleware(verifier, tempDir, response.AKSKFail())`：按 `MaxBodyBytes` 有界流式验签，同时暂存私有 0600 文件；认证成功后 rewind 才进入 handler，归因与普通中间件一致。普通 CRUD 继续使用原内存中间件。`tempDir=""` 使用系统临时目录，部署须准备与请求并发/大小匹配的磁盘容量和读取超时；正常返回、业务失败、panic 展开均清理，SIGKILL/进程崩溃残留由系统临时目录清理策略承接。请求读失败400、超限413、鉴权401、服务器暂存故障503，不回显内部路径。

非 Gin 调用方可用 `Verifier.VerifyReader(request, reader)` 流式校验；它不还原或关闭 reader，不保存 body，最多探读上限外一字节，先校验头再读体、读后重验时间窗。业务写必须在验签成功后执行。取消会在每次 Read 前后检查 context，底层阻塞 reader 的打断/关闭仍由调用方负责。新接口目前位于独立开发分支，消费者以包含它的不可变版本 pin，不代表已发布新 tag。

> 版本注记：`Verifier.Logger` 与 `response.AKSKFail()` 自 **v0.4.0** 起；归因键常量 `aksk.ContextKeyCaller`/`aksk.ContextKeyOperator`（消费方取 gin context 归因时引用，禁裸字符串）及 AKSKFail 格式提示修正自 **v0.4.1** 起，**以发布 tag 为准 pin**。

各基建包（`logger` / `postgres` / `redis` / `jwt`）自带 `Config` 结构体，零值字段取安全默认值，不绑定任何配置框架；由调用方把自己（viper / yaml / env）的配置映射进来。



## AT密码纪元（v0.4.3）


v0.4.3（关联审计 `ECO-20261104-B02-F01`）：`jwt.AccessClaims.Pwe` 与
`Manager.GenerateAccessTokenWithEpoch` 为 AT 增加签发纪元；旧签发入口保留纪元 0。
纪元存储、递增及请求时比对属于消费方职责，本包仅签发/解析。
消费方需 pin v0.4.3 或后续包含该 API 的正式版本；本地工作区联调不能替代版本 pin 验证。



## 预期结果

依赖以go.mod/go.sum固定，`go mod verify`校验通过；调用方按自己的配置框架映射各基建Config。签名失败检查服务端Logger，不向响应泄露SK或内部诊断。新Streaming/Reader消费者必须使用包含该API的精确不可变版本，不能用本机库HEAD证明发布tag具备该能力。
