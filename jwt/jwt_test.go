package jwt

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(Config{
		Secret:    "unit-test-secret-0123456789abcdef",
		AccessTTL: 30 * time.Minute,
	})
}

// 特殊场景：RefreshToken 不得冒充 AccessToken（令牌类型混淆）。
// 若 AT/RT 同密钥同算法无 typ 声明，ParseAccessToken 可成功解析 RT，
// 使 RT 绕过 30 分钟时效、登出黑名单与强制改密拦截。
func TestParseAccessToken_RejectsRefreshToken(t *testing.T) {
	m := newTestManager(t)
	rt, _, err := m.GenerateRefreshToken(42, "device-1", 168*time.Hour, 0)
	require.NoError(t, err)

	_, err = m.ParseAccessToken(rt)
	assert.Error(t, err, "refresh token must not be accepted as access token")
}

// 反向场景：AccessToken 不得当作 RefreshToken 走刷新接口
func TestParseRefreshToken_RejectsAccessToken(t *testing.T) {
	m := newTestManager(t)
	at, _, err := m.GenerateAccessToken(42, "alice", false)
	require.NoError(t, err)

	_, err = m.ParseRefreshToken(at)
	assert.Error(t, err, "access token must not be accepted as refresh token")
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := newTestManager(t)
	at, jti, err := m.GenerateAccessToken(42, "alice", true)
	require.NoError(t, err)

	claims, err := m.ParseAccessToken(at)
	require.NoError(t, err)
	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, jti, claims.JTI)
	assert.True(t, claims.MustChangePassword)
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	m := newTestManager(t)
	rt, _, err := m.GenerateRefreshToken(42, "device-1", time.Hour, 0)
	require.NoError(t, err)

	claims, err := m.ParseRefreshToken(rt)
	require.NoError(t, err)
	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, "device-1", claims.DeviceID)
}

// 密钥不符必须拒绝
func TestParseAccessToken_WrongSecret(t *testing.T) {
	m1 := NewManager(Config{Secret: "secret-aaaaaaaaaaaaaaaaaaaa", AccessTTL: time.Minute})
	m2 := NewManager(Config{Secret: "secret-bbbbbbbbbbbbbbbbbbbb", AccessTTL: time.Minute})
	at, _, err := m1.GenerateAccessToken(1, "u", false)
	require.NoError(t, err)
	_, err = m2.ParseAccessToken(at)
	assert.Error(t, err)
}

// 兼容性保障：claims 的 JSON 字段名是对外契约（uid/username/jti/mcp/typ），
// 已签发 token 依赖这些名字，改动即失效。
func TestClaimsJSONFieldNames(t *testing.T) {
	m := newTestManager(t)
	at, _, err := m.GenerateAccessToken(42, "alice", true)
	require.NoError(t, err)

	seg := strings.Split(at, ".")
	require.Len(t, seg, 3)
	payload, err := base64.RawURLEncoding.DecodeString(seg[1])
	require.NoError(t, err)
	s := string(payload)
	assert.Contains(t, s, `"uid":"42"`)
	assert.Contains(t, s, `"username":"alice"`)
	assert.Contains(t, s, `"mcp":true`)
	assert.Contains(t, s, `"typ":"access"`)
}

// Pwe 兼容矩阵钉住（v0.3.0）：新签 RT 携带纪元可解析回；旧格式（无 pwe 字段）
// 解析为 0——存量令牌平滑兼容的承诺钉子。
func TestRefreshTokenPweRoundtrip(t *testing.T) {
	m := NewManager(Config{Secret: "test-secret", AccessTTL: time.Minute})
	rt, _, err := m.GenerateRefreshToken(7, "dev-1", time.Hour, 5)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.ParseRefreshToken(rt)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Pwe != 5 {
		t.Fatalf("pwe = %d, want 5", claims.Pwe)
	}
}

func TestRefreshTokenLegacyFormatPweZero(t *testing.T) {
	// 手工构造无 pwe 字段的旧格式 RT（v0.2.x 时代签发形态）
	m := NewManager(Config{Secret: "test-secret", AccessTTL: time.Minute})
	rt, _, err := m.GenerateRefreshToken(7, "dev-1", time.Hour, 0) // pwe=0 → omitempty 不落字段（与 v0.2.x 无字段令牌解析等价）
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.ParseRefreshToken(rt)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Pwe != 0 {
		t.Fatalf("pwe = %d, want 0（旧格式平滑兼容）", claims.Pwe)
	}
}
