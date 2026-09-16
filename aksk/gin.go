// gin 集成：服务端验签中间件与出站自动签名 RoundTripper。
package aksk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// DefaultMaxBodyBytes 服务端验签中间件的默认请求体上限。
// 验签必须先完整读取 body，上限防止未认证调用方以超大请求体打服务端内存。
const DefaultMaxBodyBytes int64 = 8 << 20

// gin context 归因键（GinMiddleware 验签通过后写入；生态统一从这两键取归因，
// 2026-09-16 服务间验签统一批收编——此前 taskrunner/activelist 各自实现）。
const (
	ctxKeyCaller   = "caller"
	ctxKeyOperator = "operator"
)

// 中间件读体阶段的错误（先于验签发生，与签名正误无关）。
var (
	ErrBodyTooLarge = errors.New("aksk: request body exceeds limit")
	ErrBodyRead     = errors.New("aksk: read request body")
)

// GinMiddleware 返回验签中间件。
//
// 处理顺序：无 Authorization 头直接拒绝（不读体）→ 按 MaxBodyBytes 上限读取并
// 以内存副本还原请求体（后续 handler 的 BindJSON 不受影响）→ 验签。
// 读体失败（ErrBodyRead / ErrBodyTooLarge）与验签失败分别报错，便于排障；
// 失败现场经 Verifier.Logger 落服务端日志（默认 slog）。
//
// 验签通过后自动写入 gin context 归因键：caller = 已验签调用方 AK，
// operator = X-Operator（签名覆盖内，缺失兜底 "system"）——对齐 16 号 §9 归因口径。
//
// onFail 为 nil 时默认响应：401（验签类错误）、413（ErrBodyTooLarge）、400
// （ErrBodyRead），body 为零依赖的 {code,message,detail}；生态标准做法是传
// response.AKSKFail()（统一信封 + 分档中文文案，detail 由日志承接）。
// 需要完全自定义时传入 onFail（须自行写响应）。
func GinMiddleware(v *Verifier, onFail func(c *gin.Context, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Header.Get(HeaderAuthorization) == "" {
			fail(c, v, ErrMissingHeader, onFail)
			return
		}
		body, err := readBody(c, v.maxBody())
		if err != nil {
			fail(c, v, err, onFail)
			return
		}
		if err := v.Verify(c.Request, body); err != nil {
			fail(c, v, err, onFail)
			return
		}
		c.Set(ctxKeyCaller, CredentialOf(c.Request))
		c.Set(ctxKeyOperator, operatorOf(c.Request))
		c.Next()
	}
}

// operatorOf 取 X-Operator（签名覆盖内的身份断言），缺失兜底 "system"
// （16 号 §9 访问日志/归因统一口径）。
func operatorOf(r *http.Request) string {
	if op := r.Header.Get(HeaderXOperator); op != "" {
		return op
	}
	return "system"
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

func fail(c *gin.Context, v *Verifier, err error, onFail func(c *gin.Context, err error)) {
	// 排障现场落服务端日志（2026-09-16 统一批：响应端 detail 由日志承接）。
	// 只记方法/路径/错误文本，不落头值与签名材料。
	logger := slog.Default()
	if v != nil && v.Logger != nil {
		logger = v.Logger
	}
	logger.Warn("aksk: verify failed",
		"method", c.Request.Method, "path", c.Request.URL.Path, "err", err.Error())
	if onFail != nil {
		onFail(c, err)
		c.Abort()
		return
	}
	// message 按失败原因区分，便于服务端日志/客户端排障一眼定位；
	// detail 保留底层错误的完整上下文（含 AK、Ts 等现场信息）——零依赖默认形态，
	// 生态标准做法传 response.AKSKFail()（统一信封，无 detail 字段）。
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
