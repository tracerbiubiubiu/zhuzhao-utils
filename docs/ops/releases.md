# 版本、发布与消费者pin

本页只记录版本边界，不执行发tag或push。v0.4.3已正式发布，不能再称B02-F01未发布候选；其AT epoch API仍需消费方自行存储/比对。Reader/Streaming以及Transport读失败关闭Body修复位于审计分支候选，未因此发布v0.4.4正式tag；以包含相应实现的不可变pseudo-version和go.sum校验接入，不用main/本机HEAD替代tag对应源码。消费者是否已承接或部署由各仓收据决定。

## 已发布增量


| 版本 | 增量 |
|---|---|
| v0.4.3 | `jwt.AccessClaims.Pwe` + `GenerateAccessTokenWithEpoch`（v0.4.3已发布；旧签发入口保持纪元0）；`aksk/gin.go`·`aksk/compat/gin.go` 默认响应码值改引 `errcode` 常量（wire 形态零变化）；**哨兵/构造错误消息补包名前缀（jwt/redis/postgres/jsonutil——⚠ 消费方可感变更**：按消息文本做的日志解析或断言需同步，`errors.Is` 判定不受影响）；crypto/errcode/logger 三包补测 + jwt AT 过期哨兵用例；README 依赖边界声明修正。驱动=2026-10-15 清欠批（zhuzhao review/11 §8 BK-25） |
| v0.4.2 | `aksk/compat` 包：存量框架 `x-auth-*` 协议兼容层（签名/验签/中间件 + 出站自动签名 `Transport`，独立重实现；挂载待目标服务）；`jsonutil.Int64Slice` 补 `MarshalJSON`（发送侧 ID 数组一律字符串，配 nil→`[]` 数组恒不 null）；jwt `ParseRefreshToken` 过期错误包装 `ErrTokenExpired` 哨兵（errors.Is 可判）；postgres 移除 `MaxIdleConns` 口径误映射 |
| v0.4.1 | 归因键常量 `ContextKeyCaller/ContextKeyOperator` + AKSKFail 文案修正（头字段名 Signature=） |
| v0.4.0 | 验签统一批：`Verifier.Logger` 验签失败日志、`GinMiddleware` 归因键收编（caller/operator 写入 gin context）、`response.AKSKFail()` 标准 onFail |
| v0.3.0 | `RefreshClaims.Pwe`（密码纪元防令牌存活越权） |
| v0.2.0 | `aksk` 包：服务间 AK/SK HMAC 签名/验签（Sign/Verifier/GinMiddleware/Transport） |



## 使用与保留版本

消费方记录精确版本、tag→commit、module sum与实际源码，再运行适用门禁。候选提交若未受持久远端ref保护，本机module下载包和同盘备份只是恢复材料，不是持久发布或远端保留保证；清理审计分支前按生态收口约定处理，不由本轮文档迁移擅自发布或删除分支。

基础例子见 [接入](../guides/usage.md)，协议差异见 [API](../api/README.md)。
