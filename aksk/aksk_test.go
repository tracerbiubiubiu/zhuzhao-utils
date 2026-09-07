package aksk

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func signedRequest(t *testing.T, method, path string, body []byte, opt SignOptions) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	Sign(req, body, opt)
	return req
}

var testKeys = map[string][]byte{"zhuzhao": []byte("sk-zhuzhao")}

func TestSignVerifyOK(t *testing.T) {
	req := signedRequest(t, http.MethodPost, "/v1/tasks",
		[]byte(`{"action":"audit_archive"}`), SignOptions{
			AK: "zhuzhao", SK: testKeys["zhuzhao"],
			Operator: "10001", RequestID: "req-abc",
		})
	v := &Verifier{Keys: testKeys}
	if err := v.Verify(req, []byte(`{"action":"audit_archive"}`)); err != nil {
		t.Fatalf("expect ok, got %v", err)
	}
	// 断言头确实写入且被覆盖
	if req.Header.Get(HeaderXOperator) != "10001" || req.Header.Get(HeaderXRequestID) != "req-abc" {
		t.Fatal("operator/request-id headers not set")
	}
}

func TestVerifyErrors(t *testing.T) {
	body := []byte(`{}`)
	v := &Verifier{Keys: testKeys}
	base := SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"], Operator: "10001"}

	t.Run("wrong secret", func(t *testing.T) {
		wrong := base
		wrong.SK = []byte("sk-other")
		req := signedRequest(t, http.MethodPost, "/v1/tasks", body, wrong)
		if err := v.Verify(req, body); err != ErrBadSignature {
			t.Fatalf("want ErrBadSignature, got %v", err)
		}
	})
	t.Run("unknown credential", func(t *testing.T) {
		req := signedRequest(t, http.MethodPost, "/v1/tasks", body, SignOptions{AK: "stranger", SK: testKeys["zhuzhao"]})
		if err := v.Verify(req, body); err == nil || err.Error() == "" || !strings.Contains(err.Error(), ErrUnknownCredential.Error()) {
			t.Fatalf("want ErrUnknownCredential, got %v", err)
		}
	})
	t.Run("tampered body", func(t *testing.T) {
		req := signedRequest(t, http.MethodPost, "/v1/tasks", body, base)
		if err := v.Verify(req, []byte(`{"x":1}`)); err != ErrBadSignature {
			t.Fatalf("want ErrBadSignature, got %v", err)
		}
	})
	t.Run("tampered operator header", func(t *testing.T) {
		req := signedRequest(t, http.MethodPost, "/v1/tasks", body, base)
		req.Header.Set(HeaderXOperator, "99999") // 篡改身份断言必须失配
		if err := v.Verify(req, body); err != ErrBadSignature {
			t.Fatalf("want ErrBadSignature, got %v", err)
		}
	})
	t.Run("tampered request-id header", func(t *testing.T) {
		req := signedRequest(t, http.MethodPost, "/v1/tasks", body,
			SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"], RequestID: "req-1"})
		req.Header.Set(HeaderXRequestID, "req-2")
		if err := v.Verify(req, body); err != ErrBadSignature {
			t.Fatalf("want ErrBadSignature, got %v", err)
		}
	})
	t.Run("method mismatch", func(t *testing.T) {
		req := signedRequest(t, http.MethodPost, "/v1/tasks", body, base)
		req.Method = http.MethodPut // 签名后改方法
		if err := v.Verify(req, body); err != ErrBadSignature {
			t.Fatalf("want ErrBadSignature, got %v", err)
		}
	})
	t.Run("expired ts", func(t *testing.T) {
		past := time.Now().Add(-10 * time.Minute)
		req := signedRequest(t, http.MethodPost, "/v1/tasks", body,
			SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"], Now: func() time.Time { return past }})
		if err := v.Verify(req, body); err == nil || !strings.Contains(err.Error(), ErrExpired.Error()) {
			t.Fatalf("want ErrExpired, got %v", err)
		}
	})
	t.Run("missing header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", nil)
		if err := v.Verify(req, nil); err != ErrMissingHeader {
			t.Fatalf("want ErrMissingHeader, got %v", err)
		}
	})
	t.Run("malformed header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", nil)
		req.Header.Set(HeaderAuthorization, "Bearer abc")
		if err := v.Verify(req, nil); err != ErrBadHeader {
			t.Fatalf("want ErrBadHeader, got %v", err)
		}
	})
}

func TestGinMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	v := &Verifier{Keys: testKeys}
	r := gin.New()
	r.POST("/v1/tasks", GinMiddleware(v, nil), func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body) // 中间件还原后 handler 仍能读到完整 body
		c.JSON(200, gin.H{"len": len(b), "operator": c.GetHeader(HeaderXOperator)})
	})

	t.Run("valid signature passes and body readable", func(t *testing.T) {
		body := []byte(`{"action":"audit_archive"}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(body))
		Sign(req, body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"], Operator: "10001"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
		}
		if want := `{"len":26,"operator":"10001"}`; w.Body.String() != want {
			t.Fatalf("body = %s, want %s", w.Body.String(), want)
		}
	})
	t.Run("missing signature 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/tasks", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("want 401, got %d", w.Code)
		}
	})
	t.Run("bad signature 401", func(t *testing.T) {
		body := []byte(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(body))
		Sign(req, body, SignOptions{AK: "zhuzhao", SK: []byte("wrong")})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("want 401, got %d", w.Code)
		}
	})
}

func TestTransport(t *testing.T) {
	v := &Verifier{Keys: testKeys}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := v.Verify(r, body); err != nil {
			w.WriteHeader(401)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := &http.Client{Transport: &Transport{
		AK: "zhuzhao", SK: testKeys["zhuzhao"],
		Operator:  func(*http.Request) string { return "10001" },
		RequestID: func(*http.Request) string { return "req-xyz" },
	}}
	resp, err := client.Post(srv.URL+"/v1/tasks", "application/json", bytes.NewReader([]byte(`{"a":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, b)
	}
}

// TestRootPathSignVerify 根 URL（无路径）回调的签名一致性——签名侧 URL.Path=""
// 服务侧 "/"，canonical 归一化后必须互验通过（C9 实测暴露）。
func TestRootPathSignVerify(t *testing.T) {
	v := &Verifier{Keys: testKeys}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := v.Verify(r, body); err != nil {
			w.WriteHeader(401)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{}`)))
	Sign(req, []byte(`{}`), SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("root path sign/verify mismatch: %d %s", resp.StatusCode, b)
	}
}

// TestEmptySKTreatedAsUnknown 纵深防御：密钥环中的空 SK 视同未知凭据（配置层
// fail-closed 之外的兜底——HMAC 空密钥可被任意伪造）。
func TestEmptySKTreatedAsUnknown(t *testing.T) {
	v := &Verifier{Keys: map[string][]byte{"zhuzhao": nil}} // 空 SK 条目
	req, _ := http.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader([]byte(`{}`)))
	Sign(req, []byte(`{}`), SignOptions{AK: "zhuzhao", SK: []byte("anything")})
	if err := v.Verify(req, []byte(`{}`)); err == nil || !bytes.Contains([]byte(err.Error()), []byte(ErrUnknownCredential.Error())) {
		t.Fatalf("empty SK must be rejected as unknown credential, got %v", err)
	}
}
