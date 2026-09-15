package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config JWT 配置
type Config struct {
	Secret    string
	AccessTTL time.Duration
}

// 令牌类型声明（防止 RT 冒充 AT 的令牌类型混淆）
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// ErrTokenTypeMismatch 令牌类型不符（如用 RefreshToken 访问需 AccessToken 的接口）
var ErrTokenTypeMismatch = errors.New("token type mismatch")

// ErrTokenExpired 令牌已过期（供上层映射业务码，与「token 失效」区分）
var ErrTokenExpired = errors.New("token expired")

// AccessClaims accessToken 的 payload
type AccessClaims struct {
	UserID             int64  `json:"uid,string"`
	Username           string `json:"username"`
	JTI                string `json:"jti"`
	MustChangePassword bool   `json:"mcp,omitempty"` // 首次登录改密标记
	TokenType          string `json:"typ"`           // 必须为 TokenTypeAccess
	jwt.RegisteredClaims
}

// RefreshClaims refreshToken 的 payload
type RefreshClaims struct {
	UserID   int64  `json:"uid,string"`
	DeviceID string `json:"device_id"`
	// Pwe 密码纪元（password epoch，可选）：签发时调用方注入当前纪元值；
	// 改密/重置后调用方 INCR 纪元，Refresh 比对不一致即拒——密码重置吊销
	// 与并发 Refresh 的 TOCTOU 防线。旧令牌无此字段解析为 0，纪元从 0 起平滑兼容。
	Pwe       int64  `json:"pwe,omitempty"`
	TokenType string `json:"typ"` // 必须为 TokenTypeRefresh
	jwt.RegisteredClaims
}

// Manager JWT 签发与解析
type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewManager 创建 JWT Manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		secret:    []byte(cfg.Secret),
		accessTTL: cfg.AccessTTL,
	}
}

// GenerateAccessToken 签发 accessToken
func (m *Manager) GenerateAccessToken(userID int64, username string, mustChangePassword bool) (string, string, error) {
	now := time.Now()
	jti := generateJTI()
	claims := AccessClaims{
		UserID:             userID,
		Username:           username,
		JTI:                jti,
		MustChangePassword: mustChangePassword,
		TokenType:          TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, jti, err
}

// GenerateRefreshToken 签发 refreshToken
func (m *Manager) GenerateRefreshToken(userID int64, deviceID string, ttl time.Duration, pwe int64) (string, string, error) {
	now := time.Now()
	jti := generateJTI()
	claims := RefreshClaims{
		UserID:    userID,
		DeviceID:  deviceID,
		Pwe:       pwe,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        jti,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, jti, err
}

// ParseAccessToken 解析 accessToken；严格校验 typ 声明，拒绝 refreshToken 冒用
func (m *Manager) ParseAccessToken(tokenString string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		// 过期错误包装为本包哨兵（errors.Is 可判），上层据此返回「token 已过期」
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", ErrTokenExpired, err)
		}
		return nil, err
	}
	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	// 无 typ 或 typ 非 access 的一律拒绝（旧 token 需重新登录换取）
	if claims.TokenType != TokenTypeAccess {
		return nil, ErrTokenTypeMismatch
	}
	return claims, nil
}

// ParseRefreshToken 解析 refreshToken；严格校验 typ 声明，拒绝 accessToken 冒用
func (m *Manager) ParseRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, ErrTokenTypeMismatch
	}
	return claims, nil
}

// AccessTTL 返回 AT 的有效期
func (m *Manager) AccessTTL() time.Duration {
	return m.accessTTL
}
