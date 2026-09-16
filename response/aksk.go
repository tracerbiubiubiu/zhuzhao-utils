package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tracerbiubiubiu/zhuzhao-utils/aksk"
	"github.com/tracerbiubiubiu/zhuzhao-utils/errcode"
)

// AKSKFail 返回 aksk.GinMiddleware 的标准 onFail（2026-09-16 服务间验签统一批）：
// 读体/验签失败按统一信封响应——状态码按可修正性分档（413 缩体 / 400 连接 / 401 鉴权），
// 码值引 errcode 通用段常量，文案按失败原因分档（调用方可据此自查）；不回显错误
// 原文与签名现场，排障细节由 aksk 服务端日志（Verifier.Logger）承接。
//
// 用法：r.Use(aksk.GinMiddleware(verifier, response.AKSKFail()))。
func AKSKFail() func(c *gin.Context, err error) {
	return func(c *gin.Context, err error) {
		switch {
		case errors.Is(err, aksk.ErrBodyTooLarge):
			Fail(c, http.StatusRequestEntityTooLarge, errcode.ErrInvalidParams.Code, "请求体超过验签读体上限，请缩小后重试")
		case errors.Is(err, aksk.ErrBodyRead):
			BadRequest(c, "请求体读取失败，请检查连接后重试")
		case errors.Is(err, aksk.ErrMissingHeader):
			Unauthorized(c, "缺少 Authorization 认证头，请使用 AK/SK 签名后重试")
		case errors.Is(err, aksk.ErrBadHeader):
			Unauthorized(c, "Authorization 头格式错误，应为：HMAC Credential=<AK>, Ts=<RFC3339时间>, Signature=<hex签名>")
		case errors.Is(err, aksk.ErrUnknownCredential):
			Unauthorized(c, "访问凭证（AK）未登记或已停用，请联系管理员开通")
		case errors.Is(err, aksk.ErrExpired):
			Unauthorized(c, "请求时间戳超出允许时间窗口，请校准本机时钟后重试")
		case errors.Is(err, aksk.ErrBadSignature):
			Unauthorized(c, "签名校验失败，请检查 SK、请求体及 X-Request-ID/X-Operator 头是否与签名一致")
		default:
			Unauthorized(c, "鉴权失败，请检查 AK/SK 签名后重试")
		}
	}
}
