package compat

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// serveCompat 以参数化头/体发一个经兼容中间件的请求，返回响应记录器。
func serveCompat(v *Verifier, onFail func(*gin.Context, error), headers map[string]string, body []byte, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ingest", GinMiddleware(v, onFail), handler)
	req := httptest.NewRequest("POST", "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, val := range headers {
		req.Header.Set(k, val)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func validHeaders() map[string]string {
	return map[string]string{
		HeaderAccessKey: vecAK,
		HeaderTimestamp: vecTS,
		HeaderBodyHash:  vecHash,
		HeaderSignature: vecSig,
	}
}

func TestGinMiddleware_PassAndCallerAttribution(t *testing.T) {
	var caller string
	w := serveCompat(testVerifier(vecAt), nil, validHeaders(), vecBody, func(c *gin.Context) {
		caller = c.GetString(ContextKeyCaller)
		c.Status(http.StatusOK)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if caller != vecAK {
		t.Fatalf("caller = %q, want %q", caller, vecAK)
	}
}

// 中间件以 TrimSpace 后的 body 还原请求（对端服务端同语义），下游绑定拿到净体。
func TestGinMiddleware_RestoresTrimmedBody(t *testing.T) {
	padded := append(append([]byte{}, vecBody...), '\n')
	headers := map[string]string{
		HeaderAccessKey: vecAK,
		HeaderTimestamp: vecTS,
		HeaderSignature: vecSig,
	}
	var seen string
	w := serveCompat(testVerifier(vecAt), nil, headers, padded, func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		seen = string(b)
		c.Status(http.StatusOK)
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if seen != string(vecBody) {
		t.Fatalf("restored body = %q, want trimmed %q", seen, string(vecBody))
	}
}

func TestGinMiddleware_DefaultOnFail(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		limit   int64
		wantCd  int
	}{
		{"bad signature", map[string]string{
			HeaderAccessKey: vecAK, HeaderTimestamp: vecTS, HeaderSignature: strings.Repeat("A", 44),
		}, 0, http.StatusUnauthorized},
		{"missing headers", nil, 0, http.StatusUnauthorized},
		{"body too large", validHeaders(), 8, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := testVerifier(vecAt)
			v.MaxBodyBytes = tc.limit
			w := serveCompat(v, nil, tc.headers, vecBody, func(c *gin.Context) { c.Status(http.StatusOK) })
			if w.Code != tc.wantCd {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tc.wantCd, w.Body.String())
			}
			if tc.wantCd == http.StatusUnauthorized && !strings.Contains(w.Body.String(), "未授权") {
				t.Fatalf("default body should carry unauthorized message, got %s", w.Body.String())
			}
			// 钉住：默认响应体为固定文案，不回显错误原文与签名材料
			if strings.Contains(w.Body.String(), "compat:") || strings.Contains(w.Body.String(), vecSig) {
				t.Fatalf("default body must not echo error text or signature material, got %s", w.Body.String())
			}
		})
	}
}

func TestGinMiddleware_CustomOnFail(t *testing.T) {
	called := false
	onFail := func(c *gin.Context, err error) {
		called = true
		if !errors.Is(err, ErrBadSignature) {
			t.Errorf("onFail err = %v, want ErrBadSignature family", err)
		}
		c.JSON(http.StatusForbidden, gin.H{"custom": true})
	}
	headers := map[string]string{
		HeaderAccessKey: vecAK,
		HeaderTimestamp: vecTS,
		HeaderSignature: strings.Repeat("B", 44),
	}
	w := serveCompat(testVerifier(vecAt), onFail, headers, vecBody, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	if !called {
		t.Fatal("custom onFail not invoked")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 from custom onFail", w.Code)
	}
}

// 失败现场经 Verifier.Logger 落服务端日志（fail 路径实际使用注入的 Logger，
// 而非恒走 slog 默认出口）；日志只记 method/path/err 文本，不落头值。
func TestGinMiddleware_FailureLoggedViaVerifierLogger(t *testing.T) {
	var buf bytes.Buffer
	v := testVerifier(vecAt)
	v.Logger = slog.New(slog.NewTextHandler(&buf, nil))
	headers := map[string]string{
		HeaderAccessKey: vecAK,
		HeaderTimestamp: vecTS,
		HeaderSignature: strings.Repeat("C", 44),
	}
	w := serveCompat(v, nil, headers, vecBody, func(c *gin.Context) { c.Status(http.StatusOK) })
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	logged := buf.String()
	if !strings.Contains(logged, "compat: verify failed") || !strings.Contains(logged, "signature mismatch") {
		t.Fatalf("verify failure not logged via Verifier.Logger, log = %q", logged)
	}
}
