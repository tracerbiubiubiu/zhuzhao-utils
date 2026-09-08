package aksk

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type countingReader struct {
	r     io.Reader
	reads int
}

func (c *countingReader) Read(p []byte) (int, error) { c.reads++; return c.r.Read(p) }

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newGinWithCapture(v *Verifier) (*gin.Engine, *error) {
	gin.SetMode(gin.TestMode)
	var got error
	r := gin.New()
	r.POST("/x", GinMiddleware(v, func(c *gin.Context, err error) {
		got = err
		c.AbortWithStatus(http.StatusTeapot)
	}), func(c *gin.Context) { c.Status(200) })
	return r, &got
}

// 读体上限：超限在验签前即拒绝（ErrBodyTooLarge / 默认 413），限内正常放行。
func TestGinMiddleware_BodyLimit(t *testing.T) {
	v := &Verifier{Keys: testKeys, MaxBodyBytes: 16}

	t.Run("over limit rejected before verify", func(t *testing.T) {
		r, got := newGinWithCapture(v)
		body := bytes.Repeat([]byte("A"), 17)
		req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(body))
		Sign(req, body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]}) // 签名正确也不行
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if !errors.Is(*got, ErrBodyTooLarge) {
			t.Fatalf("want ErrBodyTooLarge, got %v", *got)
		}
	})
	t.Run("within limit passes", func(t *testing.T) {
		r, _ := newGinWithCapture(v)
		body := bytes.Repeat([]byte("A"), 16)
		req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(body))
		Sign(req, body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("want 200, got %d", w.Code)
		}
	})
	t.Run("default onFail maps to 413", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/x", GinMiddleware(v, nil), func(c *gin.Context) { c.Status(200) })
		req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(bytes.Repeat([]byte("A"), 64)))
		req.Header.Set(HeaderAuthorization, "HMAC Credential=zhuzhao, Ts=x, Signature=x")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("want 413, got %d", w.Code)
		}
	})
}

// 缺 Authorization 头时不读请求体（廉价前置拒绝）。
func TestGinMiddleware_MissingHeaderSkipsBodyRead(t *testing.T) {
	r, got := newGinWithCapture(&Verifier{Keys: testKeys})
	cr := &countingReader{r: bytes.NewReader([]byte(`{}`))}
	req := httptest.NewRequest(http.MethodPost, "/x", cr)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if *got != ErrMissingHeader {
		t.Fatalf("want ErrMissingHeader, got %v", *got)
	}
	if cr.reads != 0 {
		t.Fatalf("body should not be read, reads=%d", cr.reads)
	}
}

// 读体失败报 ErrBodyRead（默认 400），而非误导性的「签名不匹配」。
func TestGinMiddleware_ReadErrorSurfaced(t *testing.T) {
	r, got := newGinWithCapture(&Verifier{Keys: testKeys})
	body := []byte(`{"real":"payload"}`)
	signed := signedRequest(t, http.MethodPost, "/x", body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]})
	req := httptest.NewRequest(http.MethodPost, "/x", failingReader{})
	req.Header.Set(HeaderAuthorization, signed.Header.Get(HeaderAuthorization))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if !errors.Is(*got, ErrBodyRead) {
		t.Fatalf("want ErrBodyRead, got %v", *got)
	}
	if errors.Is(*got, ErrBadSignature) {
		t.Fatal("read failure must not be reported as signature mismatch")
	}

	gin.SetMode(gin.TestMode)
	r2 := gin.New()
	r2.POST("/x", GinMiddleware(&Verifier{Keys: testKeys}, nil), func(c *gin.Context) { c.Status(200) })
	req2 := httptest.NewRequest(http.MethodPost, "/x", failingReader{})
	req2.Header.Set(HeaderAuthorization, signed.Header.Get(HeaderAuthorization))
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("default onFail: want 400, got %d", w2.Code)
	}
}

// Transport 遵守 RoundTripper 契约：调用方 req 不被修改，签名落在发出的克隆上，body 一致。
func TestTransport_DoesNotMutateCallerRequest(t *testing.T) {
	v := &Verifier{Keys: testKeys}
	var sent *http.Request
	var sentBody []byte
	tr := &Transport{
		AK: "zhuzhao", SK: testKeys["zhuzhao"],
		Operator: func(*http.Request) string { return "10001" },
		Base: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			sent = r
			sentBody, _ = io.ReadAll(r.Body)
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(nil)), Request: r}, nil
		}),
	}
	body := []byte(`{"a":1}`)
	req, _ := http.NewRequest(http.MethodPost, "http://svc/v1/tasks", bytes.NewReader(body))
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get(HeaderAuthorization) != "" || req.Header.Get(HeaderXOperator) != "" {
		t.Fatal("caller request must not be mutated")
	}
	if sent == req {
		t.Fatal("transport must send a clone, not the caller's request")
	}
	if !bytes.Equal(sentBody, body) {
		t.Fatalf("sent body = %q, want %q", sentBody, body)
	}
	if err := v.Verify(sent, sentBody); err != nil {
		t.Fatalf("sent clone must carry a valid signature: %v", err)
	}
	if sent.Header.Get(HeaderXOperator) != "10001" {
		t.Fatal("operator header missing on sent clone")
	}
}
