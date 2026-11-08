# AK/SK协议与生命周期

准确入口为 `aksk/aksk.go` 的 `Sign`、`aksk/aksk.go` 的 `Verifier`、`aksk/gin.go` 的 `GinMiddleware` 和 `aksk/transport.go` 的 `Transport`；存量协议在独立 `aksk/compat` 包。最小接线见 [接入指南](../guides/usage.md)。

## 内部协议

canonical为METHOD、PATH、sha256(body)、TS、X-Request-ID、X-Operator按换行拼接；空PATH归一化 `/`。URL query不签名，完整性敏感业务参数放body。HMAC-SHA256与常量时间比较；基线±5min时间窗，不含nonce单次消费，因此不是窗口内防重放硬保证。`Verifier.MaxBodyBytes`默认8MiB；空SK条目视同未知凭据，失败关闭。

服务端先验必要头再读体，普通Gin中间件恢复body；读体失败400、超限413与验签失败401分开。`response.AKSKFail()`用标准信封和中文分档文案，Verifier.Logger保留服务端诊断。成功后归因键由中间件统一写入，调用方用ContextKeyCaller/ContextKeyOperator常量，不手写键名。

出站Transport在请求克隆上签名，不修改调用方request；读原Body失败时必须关闭原Body。请求body的生命周期和错误路径由库测试覆盖，消费方不要再复制签名/还原实现。

## 大文件与Reader

GinStreamingMiddleware按MaxBodyBytes有界读取与哈希并暂存0600私有文件；完整认证成功后rewind才进入handler。普通CRUD仍用原内存中间件。tempDir空为系统临时目录，调用方按并发×上限准备空间与超时；成功、业务失败和panic展开均清理，SIGKILL/宿主崩溃残留由系统清理策略承接。服务器暂存故障503且不泄漏内部路径。

Verifier.VerifyReader(request,reader)不还原、不关闭reader，也不保存body；最多探读上限外一字节，先验头再读体，读后重验时间窗。每次Read前后检查context；底层阻塞reader的打断/关闭仍由调用方负责。业务写必须等验签成功。这两类新接口的未发布版本边界见 [发布记录](../ops/releases.md)。

## 存量compat协议

compat是独立重实现，不依赖原库，与内部aksk零耦合，测试冻结向量为线协议依据。提供Sign/Verifier/GinMiddleware/Transport；我方补时间窗、恒时比较、读体上限、空SK拒绝。存量无operator头，归因只有caller=AK。

compat签名**不覆盖method/path**，只挂专用路由组仍防不了组内跨端点重放：组内应实质单端点或端点幂等/无副作用。无nonce的窗口内重放由业务幂等承接。Verifier.SkipBody默认false；只有抓包确认存量skipBody形态后才显式开启，默认仅真空体接受空签名。该兼容层不是新服务的默认协议，存量迁移结束后按既有决定退役。

## 密钥模型

被调方持预期调用方AK→SK小密钥环，密钥由环境/挂载注入。内部无数据库管理面。外部M2M凭据签发/存储/吊销仍是未来消费方的设计，不因本库能算HMAC就已经存在管理面；若只保存不可逆secret hash，不能据此宣称已解决HMAC验签所需密钥取得问题。

安全约束按消费方部署边界评审，不能用时间窗、内网或工具库存在推导业务exactly-once。版本和行为变更由 [发布记录](../ops/releases.md) 与精确pin决定。
