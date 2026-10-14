package compat

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 出站自动签名 → 服务端验签全链路：Transport 签出的请求必须被 Verifier 接受。
func TestTransport(t *testing.T) {
	v := &Verifier{Keys: map[string][]byte{vecAK: vecSK}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := v.Verify(r, body); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Transport: &Transport{AK: vecAK, SK: vecSK}}
	resp, err := client.Post(srv.URL+"/ingest", "application/json", bytes.NewReader(vecBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, b)
	}
}

// 无体请求（GET）同样签名可验：bodyhash 为空体哈希，落在复算派主路径上。
func TestTransport_NoBody(t *testing.T) {
	v := &Verifier{Keys: map[string][]byte{vecAK: vecSK}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := v.Verify(r, body); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Transport: &Transport{AK: vecAK, SK: vecSK}}
	resp, err := client.Get(srv.URL + "/ingest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

// Transport 遵守 RoundTripper 契约：调用方 req 不被修改，签名落在发出的克隆上，body 一致。
func TestTransport_DoesNotMutateCallerRequest(t *testing.T) {
	v := &Verifier{Keys: map[string][]byte{vecAK: vecSK}}
	var sent *http.Request
	var sentBody []byte
	tr := &Transport{
		AK: vecAK, SK: vecSK,
		Base: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			sent = r
			sentBody, _ = io.ReadAll(r.Body)
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(nil)), Request: r}, nil
		}),
	}
	body := []byte(`{"a":1}`)
	req, _ := http.NewRequest(http.MethodPost, "http://svc/ingest", bytes.NewReader(body))
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{HeaderAccessKey, HeaderTimestamp, HeaderSignature, HeaderBodyHash} {
		if req.Header.Get(h) != "" {
			t.Fatalf("caller request must not be mutated: %s set", h)
		}
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
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
