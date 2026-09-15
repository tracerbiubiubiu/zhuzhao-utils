// gin 集成：服务端验签中间件与出站自动签名 RoundTripper。
package aksk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes 服务端验签中间件的默认请求体上限。
// 验签必须先完整读取 body，上限防止未认证调用方以超大请求体打服务端内存。
const DefaultMaxBodyBytes int64 = 8 << 20

// 中间件读体阶段的错误（先于验签发生，与签名正误无关）。
var (
	ErrBodyTooLarge = errors.New("aksk: request body exceeds limit")
	ErrBodyRead     = errors.New("aksk: read request body")
)

// GinMiddleware 返回验签中间件。
//
// 处理顺序：无 Authorization 头直接拒绝（不读体）→ 按 MaxBodyBytes 上限读取并
// 以内存副本还原请求体（后续 handler 的 BindJSON 不受影响）→ 验签。
// 读体失败（ErrBodyRead / ErrBodyTooLarge）与验签失败分别报错，便于排障。
//
// onFail 为 nil 时默认响应：401（验签类错误，body 对齐 utils errcode 10002 语义）、
// 413（ErrBodyTooLarge）、400（ErrBodyRead）；本包保持零依赖不 import response。
// 需要与网关完全同构的响应结构时，调用方传入自定义 onFail（须自行写响应）。
func GinMiddleware(v *Verifier, onFail func(c *gin.Context, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Header.Get(HeaderAuthorization) == "" {
			fail(c, ErrMissingHeader, onFail)
			return
		}
		body, err := readBody(c, v.maxBody())
		if err != nil {
			fail(c, err, onFail)
			return
		}
		if err := v.Verify(c.Request, body); err != nil {
			fail(c, err, onFail)
			return
		}
		c.Next()
	}
}

// readBody 读取并还原请求体；limit <= 0 表示不限制。
func readBody(c *gin.Context, limit int64) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	r := c.Request.Body
	if limit > 0 {
		r = http.MaxBytesReader(c.Writer, r, limit)
	}
	body, err := io.ReadAll(r)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, fmt.Errorf("%w: %d bytes", ErrBodyTooLarge, mbe.Limit)
		}
		return nil, fmt.Errorf("%w: %v", ErrBodyRead, err)
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func fail(c *gin.Context, err error, onFail func(c *gin.Context, err error)) {
	if onFail != nil {
		onFail(c, err)
		c.Abort()
		return
	}
	// message 按失败原因区分，便于服务端日志/客户端排障一眼定位；
	// detail 保留底层错误的完整上下文（含 AK、Ts 等现场信息）。
	// 信封豁免注记（2026-09-15 审计）：默认失败体 {code,message,detail} 有别于
	// response 统一信封四字段（{code,message,data,request_id}）——本包刻意零依赖
	// 不 import response，且 detail 为排障现场；需完全同构时调用方自传 onFail。
	status, code, msg := http.StatusUnauthorized, 10002, "未授权：缺少签名头"
	switch {
	case errors.Is(err, ErrBodyTooLarge):
		status, code, msg = http.StatusRequestEntityTooLarge, 10001, "请求体过大"
	case errors.Is(err, ErrBodyRead):
		status, code, msg = http.StatusBadRequest, 10001, "请求体读取失败"
	case errors.Is(err, ErrBadHeader):
		msg = "未授权：签名头格式错误"
	case errors.Is(err, ErrUnknownCredential):
		msg = "未授权：未知凭据"
	case errors.Is(err, ErrExpired):
		msg = "未授权：签名已过期"
	case errors.Is(err, ErrBadSignature):
		msg = "未授权：签名不匹配"
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": msg, "detail": err.Error()})
}

// Transport 出站自动签名 RoundTripper（包装基座，如 http.DefaultTransport）。
//
// Operator / RequestID 为可选取值函数（按请求返回工号 / 关联键，返回空串则不携带）；
// 请求体被缓冲读取后再签名，实际发送内容与签名覆盖内容严格一致。
// 遵守 http.RoundTripper 契约：签名写在请求的克隆上，调用方传入的 *http.Request 不被修改。
type Transport struct {
	Base      http.RoundTripper
	AK        string
	SK        []byte
	Operator  func(*http.Request) string
	RequestID func(*http.Request) string
}

// RoundTrip 实现 http.RoundTripper。
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}
	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	signed := req.Clone(ctx)
	signed.Body = io.NopCloser(bytes.NewReader(body))
	opt := SignOptions{AK: t.AK, SK: t.SK}
	if t.Operator != nil {
		opt.Operator = t.Operator(req)
	}
	if t.RequestID != nil {
		opt.RequestID = t.RequestID(req)
	}
	Sign(signed, body, opt)
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(signed)
}
