package aksk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestVerifyReader_ProtocolAndBounds(t *testing.T) {
	body := []byte(`{"name":"流式签名"}`)
	request := signedRequest(t, http.MethodPost, "/import", body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"], Operator: "E800100", RequestID: "audit-reader"})
	for _, limit := range []int64{int64(len(body)), -1, math.MaxInt64} {
		v := &Verifier{Keys: testKeys, MaxBodyBytes: limit}
		if err := v.Verify(request, body); err != nil {
			t.Fatal(err)
		}
		if err := v.VerifyReader(request, bytes.NewReader(body)); err != nil {
			t.Fatalf("limit=%d: %v", limit, err)
		}
	}
	v := &Verifier{Keys: testKeys, MaxBodyBytes: int64(len(body) - 1)}
	if err := v.VerifyReader(request, bytes.NewReader(body)); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("want size error, got %v", err)
	}
	v.MaxBodyBytes = 0
	if err := v.VerifyReader(request, strings.NewReader("tampered")); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("want signature error, got %v", err)
	}
	empty := signedRequest(t, http.MethodGet, "/", nil, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]})
	if err := v.VerifyReader(empty, nil); err != nil {
		t.Fatal(err)
	}
	if err := v.VerifyReader(request, failingReader{}); !errors.Is(err, ErrBodyRead) {
		t.Fatalf("read errors must retain sentinel, got %v", err)
	}
	limited := http.MaxBytesReader(nil, io.NopCloser(bytes.NewReader(body)), 1)
	if err := v.VerifyReader(request, limited); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("MaxBytesReader size error lost: %v", err)
	}
}

func TestVerifyReader_RejectHeadersBeforeReading(t *testing.T) {
	for _, kind := range []string{"missing", "unknown", "expired", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			body := []byte("body")
			opts := SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]}
			want := ErrMissingHeader
			switch kind {
			case "unknown":
				opts.AK, want = "unknown", ErrUnknownCredential
			case "expired":
				opts.Now = func() time.Time { return time.Now().Add(-time.Hour) }
				want = ErrExpired
			case "malformed":
				want = ErrBadHeader
			}
			req := signedRequest(t, http.MethodPost, "/import", body, opts)
			if kind == "missing" {
				req.Header.Del(HeaderAuthorization)
			} else if kind == "malformed" {
				req.Header.Set(HeaderAuthorization, "malformed")
			}
			reader := &countingReader{r: bytes.NewReader(body)}
			err := (&Verifier{Keys: testKeys}).VerifyReader(req, reader)
			if !errors.Is(err, want) || reader.reads != 0 {
				t.Fatalf("err=%v reads=%d", err, reader.reads)
			}
		})
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestVerifyReader_ContextAndFinalTimestamp(t *testing.T) {
	body := []byte("body")
	req := signedRequest(t, http.MethodPost, "/import", body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"]})
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	cancel()
	reader := &countingReader{r: bytes.NewReader(body)}
	err := (&Verifier{Keys: testKeys}).VerifyReader(req, reader)
	if !errors.Is(err, ErrBodyRead) || !errors.Is(err, context.Canceled) || reader.reads != 0 {
		t.Fatalf("cancel err=%v reads=%d", err, reader.reads)
	}
	now := time.Now()
	req = signedRequest(t, http.MethodPost, "/import", body, SignOptions{AK: "zhuzhao", SK: testKeys["zhuzhao"], Now: func() time.Time { return now }})
	v := &Verifier{Keys: testKeys, Now: func() time.Time { return now }}
	input := bytes.NewReader(body)
	err = v.VerifyReader(req, readerFunc(func(p []byte) (int, error) {
		n, err := input.Read(p)
		now = now.Add(6 * time.Minute)
		return n, err
	}))
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("timestamp must be checked after upload, got %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// 64MiB生成流的验签堆分配须远低于文件大小，防未来退化为ReadAll。
func TestVerifyReader_LargeBodyUsesBoundedMemory(t *testing.T) {
	const size int64 = 64 << 20
	hash := sha256.New()
	if _, err := io.CopyBuffer(hash, io.LimitReader(zeroReader{}, size), make([]byte, 32<<10)); err != nil {
		t.Fatal(err)
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	now := time.Now().UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodPost, "/import", nil)
	sig := signature(testKeys["zhuzhao"], canonicalDigest(req.Method, req.URL.Path, digest, now, "", ""))
	req.Header.Set(HeaderAuthorization, fmt.Sprintf("HMAC Credential=zhuzhao, Ts=%s, Signature=%s", now, hex.EncodeToString(sig)))
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := (&Verifier{Keys: testKeys, MaxBodyBytes: size}).VerifyReader(req, io.LimitReader(zeroReader{}, size))
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 2<<20 {
		t.Fatalf("stream allocated %d bytes for %d-byte body", allocated, size)
	}
}
