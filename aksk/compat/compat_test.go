package compat

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
)

// 固定测试向量：由存量框架签名侧代码对固定 ak/sk/body 现场生成并冻结于此，
// 且已通过存量框架服务端校验器反向验证——复刻正确性的权威锚点，勿手改。
var (
	vecAK = "testak0000000000"
	vecSK = []byte("testsk0000000000000000000000000000")
	vecTS = "1791879360"
	vecAt = time.Unix(1791879360, 0)

	// V1：POST JSON 非空体（复算派主路径；服务端校验器 PASS）
	vecBody = []byte(`{"hello":"world"}`)
	vecSig  = "Dq0a+OWc7yZWlTIxeu++vkwjtWfMfsv1DJOdGGWqOwg="
	vecHash = "k6I5cakU5erL8KjSUVTNownDwccvu5kU1Hxg88toFYg="
	// V2：空体、无 body-hash 头（旧版形态：签名覆盖 bodyhash=""）
	vecSigEmpty = "xJNgdLKGXjHg3ulW5l0N8vi1Yvpe7BI4cs1G3kOLroE="
)

func signedRequest(method, target string, body []byte, sig string, withHashHeader bool) *http.Request {
	req, _ := http.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set(HeaderAccessKey, vecAK)
	req.Header.Set(HeaderTimestamp, vecTS)
	req.Header.Set(HeaderSignature, sig)
	if withHashHeader {
		req.Header.Set(HeaderBodyHash, vecHash)
	}
	return req
}

func testVerifier(now time.Time) *Verifier {
	return &Verifier{Keys: map[string][]byte{vecAK: vecSK}, Now: func() time.Time { return now }}
}

func TestVerify_GoldenV1_PeerClientVector(t *testing.T) {
	req := signedRequest("POST", "/ingest", vecBody, vecSig, true)
	if err := testVerifier(vecAt).Verify(req, vecBody); err != nil {
		t.Fatalf("golden V1 verify: %v", err)
	}
}

// 客户端哈希的是 TrimSpace 后的 body；服务端按收到体 trim 后复算——
// 发送侧带首尾空白不破坏验签。
func TestVerify_GoldenV1_BodyWithTrailingWhitespace(t *testing.T) {
	padded := append([]byte{}, vecBody...)
	padded = append(padded, '\n', ' ', '\t')
	req := signedRequest("POST", "/ingest", padded, vecSig, false)
	if err := testVerifier(vecAt).Verify(req, padded); err != nil {
		t.Fatalf("padded body verify: %v", err)
	}
}

// 旧版形态：客户端空体签 bodyhash="" 且不带头——走头原值候选通过。
func TestVerify_GoldenV2_EmptyBodyLegacyVector(t *testing.T) {
	req := signedRequest("GET", "/ingest", nil, vecSigEmpty, false)
	if err := testVerifier(vecAt).Verify(req, nil); err != nil {
		t.Fatalf("golden V2 verify: %v", err)
	}
}

func TestVerify_Failures(t *testing.T) {
	cases := []struct {
		name string
		req  *http.Request
		body []byte
		now  time.Time
		want error
	}{
		{"tampered body", signedRequest("POST", "/ingest", []byte(`{"hello":"tampered"}`), vecSig, true), []byte(`{"hello":"tampered"}`), vecAt, ErrBadSignature},
		{"empty-body sig carrying payload", signedRequest("POST", "/ingest", vecBody, vecSigEmpty, false), vecBody, vecAt, ErrBadSignature},
		{"tampered signature", signedRequest("POST", "/ingest", vecBody, strings.Repeat("A", 44), true), vecBody, vecAt, ErrBadSignature},
		{"garbage signature", signedRequest("POST", "/ingest", vecBody, "!!not-base64-hex!!", true), vecBody, vecAt, ErrBadSignature},
		{"unknown credential", func() *http.Request {
			req := signedRequest("POST", "/ingest", vecBody, vecSig, true)
			req.Header.Set(HeaderAccessKey, "other-ak")
			return req
		}(), vecBody, vecAt, ErrUnknownCredential},
		{"missing signature", func() *http.Request {
			req := signedRequest("POST", "/ingest", vecBody, vecSig, true)
			req.Header.Set(HeaderSignature, "")
			return req
		}(), vecBody, vecAt, ErrMissingHeader},
		{"malformed timestamp", func() *http.Request {
			req := signedRequest("POST", "/ingest", vecBody, vecSig, true)
			req.Header.Set(HeaderTimestamp, "not-a-number")
			return req
		}(), vecBody, vecAt, ErrBadHeader},
		{"expired beyond window", signedRequest("POST", "/ingest", vecBody, vecSig, true), vecBody, vecAt.Add(6 * time.Minute), ErrExpired},
		{"future beyond window", signedRequest("POST", "/ingest", vecBody, vecSig, true), vecBody, vecAt.Add(-6 * time.Minute), ErrExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := testVerifier(tc.now).Verify(tc.req, tc.body)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			// 门内错误以 sentinel 判定（errors.Is 语义经 %w 包装保持）
			sentinel := tc.want
			match := false
			for _, s := range []error{ErrBadSignature, ErrUnknownCredential, ErrMissingHeader, ErrBadHeader, ErrExpired} {
				if sentinel == s && strings.HasPrefix(err.Error(), s.Error()) {
					match = true
				}
			}
			if !match {
				t.Fatalf("want %v-family, got %v", tc.want, err)
			}
		})
	}
}

func TestVerify_EmptySKEntryIsUnknown(t *testing.T) {
	v := &Verifier{Keys: map[string][]byte{vecAK: {}}, Now: func() time.Time { return vecAt }}
	req := signedRequest("POST", "/ingest", vecBody, vecSig, true)
	if err := v.Verify(req, vecBody); err == nil || !strings.HasPrefix(err.Error(), ErrUnknownCredential.Error()) {
		t.Fatalf("empty SK must be rejected as unknown credential, got %v", err)
	}
}

// 对端可配 hex 编码：签名以 hex 承载同一定时结果时应可通过。
func TestVerify_HexEncodingFallback(t *testing.T) {
	sig := hex.EncodeToString(hmacSum(vecSK, vecAK, vecTS, vecHash))
	req := signedRequest("POST", "/ingest", vecBody, sig, true)
	if err := testVerifier(vecAt).Verify(req, vecBody); err != nil {
		t.Fatalf("hex-encoded signature verify: %v", err)
	}
}

func TestSign_RoundtripAndHeaders(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://x/ingest", bytes.NewReader(vecBody))
	Sign(req, vecBody, SignOptions{AK: vecAK, SK: vecSK, Now: func() time.Time { return vecAt }})

	for h, want := range map[string]string{
		HeaderAccessKey: vecAK,
		HeaderTimestamp: vecTS,
		HeaderBodyHash:  vecHash,
	} {
		if got := req.Header.Get(h); got != want {
			t.Errorf("header %s = %q, want %q", h, got, want)
		}
	}
	// 独立重算签名（与实现共用协议核，防头部组装回归）
	if got := req.Header.Get(HeaderSignature); got != vecSig {
		t.Errorf("signature = %q, want golden %q", got, vecSig)
	}
	// 验签侧 roundtrip
	if err := testVerifier(vecAt).Verify(req, vecBody); err != nil {
		t.Errorf("roundtrip verify: %v", err)
	}
}

// Sign 哈希覆盖 trim 后字节：发送侧带尾空白不影响对端复算一致。
func TestSign_TrimsBodyForHash(t *testing.T) {
	body := append([]byte{}, vecBody...)
	body = append(body, '\n')
	req, _ := http.NewRequest("POST", "http://x/ingest", bytes.NewReader(body))
	Sign(req, body, SignOptions{AK: vecAK, SK: vecSK, Now: func() time.Time { return vecAt }})
	if got := req.Header.Get(HeaderSignature); got != vecSig {
		t.Errorf("signature = %q, want golden %q (trim semantics)", got, vecSig)
	}
}

// 协议核独立对照：sort + 无分隔符拼接 + HMAC-SHA256（测试内独立实现，防实现漂移）。
func TestHmacSum_IndependentReference(t *testing.T) {
	elems := []string{vecAK, vecTS, vecHash}
	sort.Strings(elems)
	mac := hmac.New(sha256.New, vecSK)
	mac.Write([]byte(elems[0] + elems[1] + elems[2]))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if got := base64.StdEncoding.EncodeToString(hmacSum(vecSK, vecAK, vecTS, vecHash)); got != want {
		t.Fatalf("hmacSum = %s, want %s", got, want)
	}
}

// 空体签发走复算派（sha256("")），自有验签与对端服务端语义均应接受。
func TestSign_Verify_EmptyBodyRoundtrip(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://x/ingest", nil)
	Sign(req, nil, SignOptions{AK: vecAK, SK: vecSK, Now: func() time.Time { return vecAt }})
	if want := "47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="; req.Header.Get(HeaderBodyHash) != want {
		t.Fatalf("empty body hash = %q, want %q", req.Header.Get(HeaderBodyHash), want)
	}
	if err := testVerifier(vecAt).Verify(req, nil); err != nil {
		t.Fatalf("empty-body roundtrip: %v", err)
	}
}

// 时间窗边界：恰在 ±5min 内通过（含正负边界），超窗一秒即拒；自定义 MaxSkew 生效。
func TestVerify_SkewBoundaryAndCustomSkew(t *testing.T) {
	req := signedRequest("POST", "/ingest", vecBody, vecSig, true)
	for _, d := range []time.Duration{0, DefaultMaxSkew, -DefaultMaxSkew} {
		if err := testVerifier(vecAt.Add(d)).Verify(req, vecBody); err != nil {
			t.Errorf("skew %s: want pass, got %v", d, err)
		}
	}
	if err := testVerifier(vecAt.Add(DefaultMaxSkew+time.Second)).Verify(req, vecBody); err == nil {
		t.Error("beyond default window: want fail")
	}
	v := testVerifier(vecAt)
	v.MaxSkew = time.Minute
	if err := v.Verify(req, vecBody); err != nil {
		t.Fatalf("exact ts with custom skew: %v", err)
	}
	v2 := testVerifier(vecAt.Add(2 * time.Minute))
	v2.MaxSkew = time.Minute
	if err := v2.Verify(req, vecBody); err == nil {
		t.Error("beyond custom 1min window: want fail")
	}
}

func TestCredentialOf(t *testing.T) {
	req := signedRequest("POST", "/ingest", vecBody, vecSig, true)
	if got := CredentialOf(req); got != vecAK {
		t.Fatalf("CredentialOf = %q, want %q", got, vecAK)
	}
}
