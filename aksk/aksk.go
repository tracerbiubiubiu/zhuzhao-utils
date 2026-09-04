// Package aksk 提供内部服务间通信的 AK/SK HMAC-SHA256 签名与验签。
//
// 设计依据：zhuzhao 仓库 docs/phase3/16-external-integration.md §9 公共能力统一基线。
// 密钥模型：按调用方发 SK（如 zhuzhao / taskrunner / activelist 各一把，env 注入各自容器）；
// 被调方持有「预期调用方」的 AK→SK 小密钥环验签。内部静态密钥、无管理面；
// 外部 M2M 凭据（DB 存储/哈希/签发吊销）属另一层，算法层复用本包。
//
// 签名头格式：
//
//	Authorization: HMAC Credential=<ak>, Ts=<RFC3339 UTC>, Signature=<hex>
//
// 待签名串（canonical，逐字段以 \n 连接）：
//
//	METHOD \n PATH \n sha256hex(body) \n Ts \n X-Request-ID \n X-Operator
//
// X-Request-ID / X-Operator 为可选属性（可为空串），一旦携带即纳入签名覆盖——
// 验签方读取请求头中的值参与重算，故**篡改头会使签名失配**（身份断言不可伪造）。
// 防重放：Ts 与验签方时钟偏差超过时间窗（默认 ±5min）即拒绝；内部低频调用不引入 nonce。
package aksk

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"net/http"
	"strings"
	"time"
)

// 参与签名 / 验签的请求头。
const (
	HeaderAuthorization = "Authorization"
	HeaderXRequestID    = "X-Request-ID"
	HeaderXOperator     = "X-Operator"

	scheme = "HMAC" // Authorization 头的认证方案前缀
)

// 验签错误。错误信息面向排障（内部网络），不区分「密钥不存在/签名错误」的对外语义。
var (
	ErrMissingHeader     = errors.New("aksk: missing Authorization header")
	ErrBadHeader         = errors.New("aksk: malformed Authorization header")
	ErrUnknownCredential = errors.New("aksk: unknown credential")
	ErrExpired           = errors.New("aksk: timestamp out of allowed window")
	ErrBadSignature      = errors.New("aksk: signature mismatch")
)

// DefaultMaxSkew 默认时间窗。
const DefaultMaxSkew = 5 * time.Minute

// SignOptions 出站签名选项。SK 为调用方密钥；Operator / RequestID 非空时写入对应头（纳入签名覆盖）。
type SignOptions struct {
	AK        string
	SK        []byte
	Operator  string
	RequestID string
	Now       func() time.Time // 可注入时钟（测试用）
}

// Sign 为出站请求写入 X-Operator / X-Request-ID（若提供）与 Authorization 签名头。
// body 为请求体字节（与实际发送内容一致；nil 按空体签名）。
func Sign(req *http.Request, body []byte, opt SignOptions) {
	now := time.Now
	if opt.Now != nil {
		now = opt.Now
	}
	if opt.Operator != "" {
		req.Header.Set(HeaderXOperator, opt.Operator)
	}
	if opt.RequestID != "" {
		req.Header.Set(HeaderXRequestID, opt.RequestID)
	}
	ts := now().UTC().Format(time.RFC3339)
	sig := hex.EncodeToString(signature(opt.SK, canonical(
		req.Method, req.URL.Path, body, ts,
		req.Header.Get(HeaderXRequestID), req.Header.Get(HeaderXOperator),
	)))
	req.Header.Set(HeaderAuthorization,
		fmt.Sprintf("%s Credential=%s, Ts=%s, Signature=%s", scheme, opt.AK, ts, sig))
}

// Verifier 服务端验签器。Keys 为预期调用方的 AK → SK 映射（小密钥环）。
// MaxSkew 零值取 DefaultMaxSkew；Now 可注入时钟（测试用）。
type Verifier struct {
	Keys    map[string][]byte
	MaxSkew time.Duration
	Now     func() time.Time
}

// Verify 校验请求签名。body 为请求体字节（中间件已读取还原，调用方再次绑定不受影响）。
func (v *Verifier) Verify(r *http.Request, body []byte) error {
	h := r.Header.Get(HeaderAuthorization)
	if h == "" {
		return ErrMissingHeader
	}
	ak, ts, sig, err := parseAuth(h)
	if err != nil {
		return err
	}
	sk, ok := v.Keys[ak]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownCredential, ak)
	}
	stamp, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return fmt.Errorf("%w: bad ts %q", ErrBadHeader, ts)
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	skew := v.MaxSkew
	if skew <= 0 {
		skew = DefaultMaxSkew
	}
	if d := now().Sub(stamp); d > skew || d < -skew {
		return fmt.Errorf("%w: %s", ErrExpired, ts)
	}
	want := signature(sk, canonical(
		r.Method, r.URL.Path, body, ts,
		r.Header.Get(HeaderXRequestID), r.Header.Get(HeaderXOperator),
	))
	got, err := hex.DecodeString(sig)
	if err != nil || subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrBadSignature
	}
	return nil
}

// canonical 组装待签名串。requestID / operator 与请求头中的值一致（可能为空串）。
func canonical(method, path string, body []byte, ts, requestID, operator string) []byte {
	sum := sha256.Sum256(body)
	return []byte(strings.Join([]string{
		method, path, hex.EncodeToString(sum[:]), ts, requestID, operator,
	}, "\n"))
}

// signature HMAC-SHA256 摘要（未编码）。
func signature(sk, canonicalString []byte) []byte {
	mac := hmac.New(func() hash.Hash { return sha256.New() }, sk)
	mac.Write(canonicalString)
	return mac.Sum(nil)
}

// parseAuth 解析 "HMAC Credential=<ak>, Ts=<ts>, Signature=<sig>"。
func parseAuth(h string) (ak, ts, sig string, err error) {
	rest, ok := strings.CutPrefix(h, scheme+" ")
	if !ok {
		return "", "", "", ErrBadHeader
	}
	for _, part := range strings.Split(rest, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return "", "", "", ErrBadHeader
		}
		switch k {
		case "Credential":
			ak = v
		case "Ts":
			ts = v
		case "Signature":
			sig = v
		}
	}
	if ak == "" || ts == "" || sig == "" {
		return "", "", "", ErrBadHeader
	}
	return ak, ts, sig, nil
}
