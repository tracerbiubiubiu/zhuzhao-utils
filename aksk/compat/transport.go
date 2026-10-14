// 出站自动签名 RoundTripper：形态对齐内部协议（../aksk）的 Transport，
// 签名核为存量协议（x-auth-* 四头，见 compat.go）。
package compat

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// Transport 出站自动签名 RoundTripper（挂在 http.Client 上，发往存量框架的
// 请求自动携带 x-auth-* 四头）：
//
//	client := &http.Client{Transport: &compat.Transport{AK: ak, SK: sk}}
//
// 请求体被缓冲读取后再签名，实际发送内容与签名覆盖内容严格一致；签名写在
// 请求的克隆上，调用方传入的 *http.Request 不被修改（RoundTripper 契约）。
// Base 为底层传输，零值取 http.DefaultTransport；Now 可注入时钟（测试用）。
type Transport struct {
	Base http.RoundTripper
	AK   string
	SK   []byte
	Now  func() time.Time
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
	signed := req.Clone(req.Context())
	signed.Body = io.NopCloser(bytes.NewReader(body))
	Sign(signed, body, SignOptions{AK: t.AK, SK: t.SK, Now: t.Now})
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(signed)
}
