package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccessTokenPasswordEpoch(t *testing.T) {
	m := NewManager(Config{Secret: "epoch-test-secret", AccessTTL: time.Minute})
	legacy, _, err := m.GenerateAccessToken(42, "alice", false)
	require.NoError(t, err)
	claims, err := m.ParseAccessToken(legacy)
	require.NoError(t, err)
	require.Zero(t, claims.Pwe, "旧签发入口保持纪元 0 兼容")

	current, _, err := m.GenerateAccessTokenWithEpoch(42, "alice", true, 7)
	require.NoError(t, err)
	claims, err = m.ParseAccessToken(current)
	require.NoError(t, err)
	require.EqualValues(t, 7, claims.Pwe)
	require.True(t, claims.MustChangePassword)
	require.EqualValues(t, 42, claims.UserID)
	_, err = m.ParseRefreshToken(current)
	require.ErrorIs(t, err, ErrTokenTypeMismatch)

	_, _, err = m.GenerateAccessTokenWithEpoch(42, "alice", false, -1)
	require.Error(t, err)
}
