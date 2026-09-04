// gin 集成：服务端验签中间件与出站自动签名 RoundTripper。
package aksk

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GinMiddleware 返回验签中间件。
//
// 请求体会被完整读取参与签名后以内存副本还原，后续 handler 的 BindJSON 不受影响。
// onFail 为 nil 时默认 401（body 对齐 utils errcode 的 10002 语义，本包保持零依赖不 import response）；
// 需要与网关完全同构的响应结构时，调用方传入使用 response.Unauthorized 的自定义 onFail。
func GinMiddleware(v *Verifier, onFail func(c *gin.Context, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}
		if err := v.Verify(c.Request, body); err != nil {
			if onFail != nil {
				onFail(c, err)
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"code": 10002, "message": "未授权", "detail": err.Error(),
				})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

// Transport 出站自动签名 RoundTripper（包装基座，如 http.DefaultTransport）。
//
// Operator / RequestID 为可选取值函数（按请求返回工号 / 关联键，返回空串则不携带）；
// 请求体被缓冲读取后还原再签名，实际发送内容与签名覆盖内容严格一致。
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
		req.Body = io.NopCloser(bytes.NewBuffer(body))
	}
	opt := SignOptions{AK: t.AK, SK: t.SK}
	if t.Operator != nil {
		opt.Operator = t.Operator(req)
	}
	if t.RequestID != nil {
		opt.RequestID = t.RequestID(req)
	}
	Sign(req, body, opt)
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
