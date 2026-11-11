package aksk_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/tracerbiubiubiu/zhuzhao-utils/aksk"
	"github.com/tracerbiubiubiu/zhuzhao-utils/aksk/compat"
)

type closeProbeBody struct {
	io.Reader
	readErr, closeErr error
	closes            int
}

func (b *closeProbeBody) Read(p []byte) (int, error) {
	if b.readErr != nil {
		return 0, b.readErr
	}
	return b.Reader.Read(p)
}
func (b *closeProbeBody) Close() error { b.closes++; return b.closeErr }

type closeProbeTransport func(*http.Request) (*http.Response, error)

func (f closeProbeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type signatureVerifier interface {
	Verify(*http.Request, []byte) error
}

func TestTransportBodyLifecycle(t *testing.T) {
	ak, sk := "audit", []byte("audit-key")
	for _, protocol := range []struct {
		name   string
		wrap   func(http.RoundTripper) http.RoundTripper
		verify signatureVerifier
	}{
		{"internal", func(base http.RoundTripper) http.RoundTripper { return &aksk.Transport{Base: base, AK: ak, SK: sk} }, &aksk.Verifier{Keys: map[string][]byte{ak: sk}}},
		{"compat", func(base http.RoundTripper) http.RoundTripper { return &compat.Transport{Base: base, AK: ak, SK: sk} }, &compat.Verifier{Keys: map[string][]byte{ak: sk}}},
	} {
		t.Run(protocol.name, func(t *testing.T) {
			readErr, closeErr, sendErr := errors.New("read sentinel"), errors.New("close sentinel"), errors.New("send sentinel")
			for _, tc := range []struct {
				name                       string
				readErr, closeErr, sendErr error
			}{
				{"success", nil, nil, nil}, {"read_error", readErr, nil, nil},
				{"read_and_close_error", readErr, closeErr, nil}, {"close_error_after_read", nil, closeErr, nil},
				{"send_error", nil, nil, sendErr},
			} {
				t.Run(tc.name, func(t *testing.T) {
					body := []byte(`{"value":"signed"}`)
					input := &closeProbeBody{Reader: bytes.NewReader(body), readErr: tc.readErr, closeErr: tc.closeErr}
					called := 0
					base := closeProbeTransport(func(r *http.Request) (*http.Response, error) {
						called++
						defer r.Body.Close()
						got, err := io.ReadAll(r.Body)
						if err != nil {
							t.Fatal(err)
						}
						if !bytes.Equal(got, body) {
							t.Fatalf("sent body=%q", got)
						}
						if err := protocol.verify.Verify(r, got); err != nil {
							t.Fatalf("sent signature: %v", err)
						}
						if tc.sendErr != nil {
							return nil, tc.sendErr
						}
						return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
					})
					req, _ := http.NewRequest(http.MethodPost, "http://service.invalid/ingest", input)
					_, err := protocol.wrap(base).RoundTrip(req)
					wantErr := tc.readErr
					if wantErr == nil {
						wantErr = tc.sendErr
					}
					if !errors.Is(err, wantErr) {
						t.Fatalf("original operation error lost: got=%v want=%v", err, wantErr)
					}
					if input.closes != 1 {
						t.Fatalf("original body must close once, including read error: closes=%d", input.closes)
					}
					if (called == 0) != (tc.readErr != nil) {
						t.Fatalf("base calls=%d readErr=%v", called, tc.readErr)
					}
				})
			}
		})
	}
}
