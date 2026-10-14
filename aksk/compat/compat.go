// Package compat 提供某存量框架 AK/SK 签名协议的兼容层（出站签名 + 入站验签）。
//
// 出处说明：协议形态经研读存量项目实现后独立重实现——未复制其源码，本包的
// 结构、命名与测试均为原创；线协议本身以对端部署物为权威，边界以本包测试
// 冻结的向量为准。
//
// 线协议由对端部署物决定，属冻结协议——不随本包演进，也不与内部协议互通：
//
//	头：x-auth-access-key / x-auth-timestamp(Unix 秒) / x-auth-signature / x-auth-body-hash
//	bodyhash  = base64( sha256( TrimSpace(body) ) )   （"复算派"主路径）
//	待签串    = sort([ak, ts, bodyhash]) 无分隔符拼接
//	signature = base64( HMAC-SHA256(sk, 待签串) )      （验签侧兼容 hex 编码）
//
// 与内部协议（../aksk）完全隔离：不共享代码、密钥或路由。使用纪律：兼容验签
// 中间件只挂专用路由组，不与内部 GinMiddleware 混挂同组——存量签名不覆盖
// method/path，混挂等于扩大跨端点重放面；且路由组只隔离组外，组内跨端点重放
// 依然成立，该组须实质单端点，或组内端点幂等/无副作用。
//
// 我方端补强（对端不校验，本包单方面生效）：时间窗 ±5min（默认）、恒时比较、
// 读体上限、失败落日志不回显签名材料、验签通过 caller=AK 归因。
//
// 防重放取舍与内部基线一致（phase3/16 §9）：仅时间窗、不做 nonce——窗口内
// 残余重放由业务幂等承接。
//
// 能力对账（2026-10-13，对端四个相关包全量枚举）：线协议/签名/验签/中间件
// 能力全量迁移；出站自动签名 Transport（2026-10-14 补）为我方消费侧增补——
// 形态对齐内部 aksk.Transport，供我方服务调用存量接口时自动签名，不改变线协议。
// 有意排除=对端密钥管理面（AK/SK 生成、Mongo 明文存储、签发
// 端点）、HTTP 200 包装错误与 event_id 依赖、绑定其存储的一站式默认中间件；
// 形态差异=密钥注入固定为 config 密钥环（KeyGetter 函数形态未预留，DB 动态
// 密钥场景触发时补）、Sign 无构造期空密钥校验（config 层 fail-closed 承接）。
//
// bodyhash 验签语义：主路径恒按复算派（base64(sha256(TrimSpace(body)))，body
// 完整性在签名覆盖内，x-auth-body-hash 头仅由签名侧写出、验签侧不消费——与
// 对端当前版服务端一致）；另兼容存量 skipBody 形态签名（待签 bodyhash 为 ""）：
// 默认仅当 body（trim 后）真空时接受——空签名不可携带任意体重放；Verifier
// .SkipBody=true（对端 Config.SkipBody 同名同语义）后无条件接受，body 完整性
// 让渡由开关持有方知情承担。
package compat

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 参与签名 / 验签的请求头（存量协议冻结命名）。
const (
	HeaderAccessKey = "x-auth-access-key"
	HeaderTimestamp = "x-auth-timestamp"
	HeaderSignature = "x-auth-signature"
	HeaderBodyHash  = "x-auth-body-hash"
)

const (
	// DefaultMaxSkew 默认时间窗（对端不校验时间戳，我方强制补上防重放）。
	DefaultMaxSkew = 5 * time.Minute
	// DefaultMaxBodyBytes 服务端验签中间件的默认请求体上限：验签必须先完整
	// 读取 body，上限防止未认证调用方以超大请求体打服务端内存。
	DefaultMaxBodyBytes int64 = 8 << 20
)

// gin context 归因键，与内部协议（../aksk）同名对齐，供 handler 侧统一取法。
const ContextKeyCaller = "caller"

// 验签错误。文案面向服务端排障日志，不区分对外语义。
var (
	ErrMissingHeader     = errors.New("compat: missing access-key or signature header")
	ErrBadHeader         = errors.New("compat: malformed timestamp header")
	ErrUnknownCredential = errors.New("compat: unknown credential")
	ErrExpired           = errors.New("compat: timestamp out of allowed window")
	ErrBadSignature      = errors.New("compat: signature mismatch")
)

// SignOptions 出站签名选项。SK 为调用方密钥；Now 可注入时钟（测试用）。
//
// 签名侧统一按复算派：bodyhash 恒为 base64(sha256(TrimSpace(body)))（空体同式，
// 与对端服务端按收到体复算的值一致）。哈希只覆盖 TrimSpace 后的字节，调用方
// 发送带首尾空白的 body 亦可（对端验签前同样 trim）。
type SignOptions struct {
	AK  string
	SK  []byte
	Now func() time.Time
}

// Sign 为出站请求写入 x-auth-* 四头。body 须与实际发送内容一致。
func Sign(req *http.Request, body []byte, opt SignOptions) {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	ts := strconv.FormatInt(opt.Now().Unix(), 10)
	bodyHash := bodyHashOf(body)
	sig := base64.StdEncoding.EncodeToString(hmacSum(opt.SK, opt.AK, ts, bodyHash))
	req.Header.Set(HeaderAccessKey, opt.AK)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderBodyHash, bodyHash)
	req.Header.Set(HeaderSignature, sig)
}

// Verifier 服务端验签器。Keys 为存量调用方 AK → SK 映射（config 密钥环注入，
// 不对接对端存储）。MaxSkew 零值取 DefaultMaxSkew；Now 可注入时钟（测试用）。
// MaxBodyBytes 为 GinMiddleware 读体上限：零值取 DefaultMaxBodyBytes。
// Logger 为验签失败日志出口；零值走 slog 默认。
type Verifier struct {
	Keys         map[string][]byte
	MaxSkew      time.Duration
	Now          func() time.Time
	MaxBodyBytes int64
	Logger       *slog.Logger

	// SkipBody 兼容对端存量开关（对端中间件 Config.SkipBody 同名同语义）：
	// true = 额外接受签名覆盖 bodyhash 为 "" 的请求（body 不在签名覆盖内，
	// 存量 skipBody 客户端形态）；默认 false 仅 body 恒入签（"" 候选限真空体）。
	// 开启即让渡 body 完整性——截获的一条空签名可在窗口内携带任意体重放，
	// 须限定专用路由组并知情接受；抓包确认存量流量确有此形态后再开。
	SkipBody bool
}

func (v *Verifier) maxBody() int64 {
	if v.MaxBodyBytes == 0 {
		return DefaultMaxBodyBytes
	}
	return v.MaxBodyBytes
}

// CredentialOf 从请求头读取调用方 AK（GinMiddleware 验签通过后已写入 gin
// context "caller"；非 gin 场景用本函数）。
func CredentialOf(r *http.Request) string { return r.Header.Get(HeaderAccessKey) }

// Verify 校验请求签名。body 为请求体字节（中间件已读取还原，后续绑定不受影响）。
func (v *Verifier) Verify(r *http.Request, body []byte) error {
	ak := r.Header.Get(HeaderAccessKey)
	sig := r.Header.Get(HeaderSignature)
	if ak == "" || sig == "" {
		return ErrMissingHeader
	}
	sk, ok := v.Keys[ak]
	// 空 SK 条目 = HMAC 空密钥可伪造，视同未知凭据（配置层也应 fail-closed 拒载）
	if !ok || len(sk) == 0 {
		return fmt.Errorf("%w: %s", ErrUnknownCredential, ak)
	}
	ts := r.Header.Get(HeaderTimestamp)
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: ts %q is not unix seconds", ErrBadHeader, ts)
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	skew := v.MaxSkew
	if skew <= 0 {
		skew = DefaultMaxSkew
	}
	if d := now().Sub(time.Unix(sec, 0)); d > skew || d < -skew {
		return fmt.Errorf("%w: ts %s", ErrExpired, ts)
	}
	got := decodeSignatures(sig)
	if len(got) == 0 {
		return fmt.Errorf("%w: not base64/hex", ErrBadSignature)
	}
	for _, candidate := range v.sigCandidates(body) {
		for _, mac := range got {
			if hmac.Equal(mac, hmacSum(sk, ak, ts, candidate)) {
				return nil
			}
		}
	}
	return fmt.Errorf("%w (ak=%s)", ErrBadSignature, ak)
}

// sigCandidates 返回验签候选：①复算派 bodyhash（主路径，body 恒入签）②""
// （存量 skipBody 形态签名）。默认仅真空体接受 ②（空签名无法携带 payload
// 重放）；Verifier.SkipBody=true 后 ② 无条件参与——body 完整性让渡由开关
// 持有方知情承担。
func (v *Verifier) sigCandidates(body []byte) []string {
	if v.SkipBody || len(bytes.TrimSpace(body)) == 0 {
		return []string{bodyHashOf(body), ""}
	}
	return []string{bodyHashOf(body)}
}

// bodyHashOf 复算派 bodyhash：TrimSpace 后 sha256 再 base64。
func bodyHashOf(body []byte) string {
	sum := sha256.Sum256(bytes.TrimSpace(body))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// hmacSum 存量协议签名核：sort([ak, ts, bodyHash]) 无分隔符拼接后 HMAC-SHA256。
func hmacSum(sk []byte, ak, ts, bodyHash string) []byte {
	elems := []string{ak, ts, bodyHash}
	sort.Strings(elems)
	mac := hmac.New(sha256.New, sk)
	mac.Write([]byte(strings.Join(elems, "")))
	return mac.Sum(nil)
}

// decodeSignatures 返回签名的全部合法解码形态：默认 base64；hex 为对端可配
// 编码。hex 串同时是合法 base64（字符集为子集），两解都参与比较、按编码唯一性
// 自然互斥，不做先序偏袒。
func decodeSignatures(sig string) [][]byte {
	var out [][]byte
	if b, err := base64.StdEncoding.DecodeString(sig); err == nil {
		out = append(out, b)
	}
	if b, err := hex.DecodeString(sig); err == nil {
		out = append(out, b)
	}
	return out
}
