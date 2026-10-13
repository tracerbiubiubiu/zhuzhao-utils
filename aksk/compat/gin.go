// gin 集成：存量协议验签中间件。与内部协议（../aksk）的 GinMiddleware 形态对齐，
// 但路由挂载物理隔离（见包注释使用纪律）。
package compat

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// 中间件读体阶段的错误（先于验签发生，与签名正误无关）。
var (
	ErrBodyTooLarge = errors.New("compat: request body exceeds limit")
	ErrBodyRead     = errors.New("compat: read request body")
)

// GinMiddleware 返回存量协议验签中间件。
//
// 处理顺序：无 AK/签名头直接拒绝（不读体）→ 按上限读取 body 并 TrimSpace 后
// 还原请求体（后续 handler 的 BindJSON 不受影响；对端服务端同样以 trim 后的
// body 参与复算与下发）→ 验签 → 写 gin context caller=AK。
//
// onFail 为 nil 时默认响应 401 + {code,message}（不回显错误原文）；需要分档
// 文案时传入自定义 onFail（须自行写响应）。
func GinMiddleware(v *Verifier, onFail func(c *gin.Context, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Header.Get(HeaderAccessKey) == "" || c.Request.Header.Get(HeaderSignature) == "" {
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
		c.Set(ContextKeyCaller, CredentialOf(c.Request))
		c.Next()
	}
}

// readBody 读取、TrimSpace 并还原请求体；limit <= 0 表示不限制。
// trim 与对端服务端语义一致：其验签与下发均基于 trim 后的 body。
func readBody(c *gin.Context, limit int64) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	r := c.Request.Body
	if limit > 0 {
		r = http.MaxBytesReader(c.Writer, r, limit)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, fmt.Errorf("%w: %d bytes", ErrBodyTooLarge, mbe.Limit)
		}
		return nil, fmt.Errorf("%w: %v", ErrBodyRead, err)
	}
	body := bytes.TrimSpace(raw)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func fail(c *gin.Context, v *Verifier, err error, onFail func(c *gin.Context, err error)) {
	// 排障现场落服务端日志；只记方法/路径/错误文本，不落头值与签名材料。
	logger := slog.Default()
	if v != nil && v.Logger != nil {
		logger = v.Logger
	}
	logger.Warn("compat: verify failed",
		"method", c.Request.Method, "path", c.Request.URL.Path, "err", err.Error())
	if onFail != nil {
		onFail(c, err)
		c.Abort()
		return
	}
	status := http.StatusUnauthorized
	if errors.Is(err, ErrBodyTooLarge) {
		status = http.StatusRequestEntityTooLarge
	} else if errors.Is(err, ErrBodyRead) {
		status = http.StatusBadRequest
	}
	c.AbortWithStatusJSON(status, gin.H{"code": 10002, "message": "未授权：AK/SK 签名验证失败"})
}
