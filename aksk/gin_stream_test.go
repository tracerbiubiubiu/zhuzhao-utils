package aksk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGinStreamingMiddleware_Lifecycle(t *testing.T) {
	for _, mode := range []string{"success", "handler-fail", "panic", "bad-signature", "over-limit", "read-failure", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			body := []byte("streamed synthetic body")
			req := signedRequest(t, http.MethodPost, "/import", body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"], Operator: "E800110"})
			req.Body = io.NopCloser(bytes.NewReader(body))
			if mode == "bad-signature" {
				req.Body = io.NopCloser(strings.NewReader("different"))
			}
			if mode == "read-failure" {
				req.Body = io.NopCloser(failingReader{})
			}
			if mode == "cancelled" {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			v := &Verifier{Keys: testKeys, MaxBodyBytes: int64(len(body))}
			if mode == "over-limit" {
				v.MaxBodyBytes--
			}
			entered := false
			r := gin.New()
			r.Use(gin.Recovery())
			r.POST("/import", GinStreamingMiddleware(v, dir, nil), func(c *gin.Context) {
				entered = true
				files, err := os.ReadDir(dir)
				if err != nil || len(files) != 1 {
					t.Fatalf("spool not present: %v %v", files, err)
				}
				stat, err := os.Stat(filepath.Join(dir, files[0].Name()))
				if err != nil {
					t.Fatal(err)
				}
				if stat.Mode().Perm() != 0600 {
					t.Fatalf("spool permission: %v", stat.Mode())
				}
				got, err := io.ReadAll(c.Request.Body)
				if err != nil || !bytes.Equal(got, body) {
					t.Fatalf("body restore failed: %q %v", got, err)
				}
				if c.GetString(ContextKeyCaller) != "zhuzhao" || c.GetString(ContextKeyOperator) != "E800110" {
					t.Fatal("verified attribution missing")
				}
				if mode == "panic" {
					panic("synthetic handler panic")
				}
				if mode == "handler-fail" {
					c.AbortWithStatus(422)
					return
				}
				c.Status(200)
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			want := map[string]int{"success": 200, "handler-fail": 422, "panic": 500, "bad-signature": 401, "over-limit": 413, "read-failure": 400, "cancelled": 400}[mode]
			if w.Code != want {
				t.Fatalf("status=%d want=%d", w.Code, want)
			}
			if entered != (mode == "success" || mode == "handler-fail" || mode == "panic") {
				t.Fatal("handler authentication gate mismatch")
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("spool leaked: %v %v", files, err)
			}
		})
	}
}

func TestGinStreamingMiddleware_InvalidHeadersDoNotSpool(t *testing.T) {
	// Invalid temp directory makes any attempt to spool fail as 503. Header errors
	// must instead be rejected before reading or touching temporary storage.
	for _, mode := range []string{"missing", "unknown", "expired", "short-signature"} {
		t.Run(mode, func(t *testing.T) {
			opts := SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]}
			if mode == "unknown" {
				opts.AK = "not-registered"
			}
			req := signedRequest(t, http.MethodPost, "/import", nil, opts)
			if mode == "missing" {
				req.Header.Del(HeaderAuthorization)
			}
			if mode == "expired" {
				req.Header.Set(HeaderAuthorization, "HMAC Credential=zhuzhao, Ts=2020-01-01T00:00:00Z, Signature="+strings.Repeat("0", 64))
			}
			if mode == "short-signature" {
				req.Header.Set(HeaderAuthorization, "HMAC Credential=zhuzhao, Ts="+strings.Split(req.Header.Get(HeaderAuthorization), "Ts=")[1][:20]+", Signature=00")
			}
			input := &countingReader{r: strings.NewReader("not read")}
			req.Body = io.NopCloser(input)
			r := gin.New()
			r.POST("/import", GinStreamingMiddleware(&Verifier{Keys: testKeys}, filepath.Join(t.TempDir(), "missing"), nil), func(c *gin.Context) { t.Fatal("unauthorized handler") })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 401 || input.reads != 0 {
				t.Fatalf("status=%d reads=%d", w.Code, input.reads)
			}
		})
	}
}

func TestGinStreamingMiddleware_StorageErrors(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "file")
	if err := os.WriteFile(invalid, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	req := signedRequest(t, http.MethodPost, "/import", nil, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]})
	r := gin.New()
	r.POST("/import", GinStreamingMiddleware(&Verifier{Keys: testKeys}, invalid, nil), func(c *gin.Context) { t.Fatal("storage failure entered handler") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatalf("storage failure status %d", w.Code)
	}
	diskErr := errors.New("synthetic disk full")
	_, err := (streamStoreWriter{writerFunc(func([]byte) (int, error) { return 0, diskErr })}).Write([]byte("x"))
	if !errors.Is(err, ErrBodyStore) || !errors.Is(err, diskErr) {
		t.Fatalf("storage cause lost: %v", err)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func ExampleGinStreamingMiddleware() {
	// Only the large-body route uses disk spooling. The verifier retains the same
	// HMAC protocol, bounded memory, and caller/operator identity as normal routes.
	r := gin.New()
	v := &Verifier{Keys: map[string][]byte{"caller": []byte("secret")}, MaxBodyBytes: 1 << 30}
	r.POST("/import", GinStreamingMiddleware(v, "", nil), func(c *gin.Context) { c.Status(200) })
	fmt.Println(len(r.Routes()))
	// Output: 1
}
