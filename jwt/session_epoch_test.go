package jwt

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionEpochTokens(t *testing.T) {
	manager := NewManager(Config{Secret: "synthetic-session-epoch-test-key", AccessTTL: time.Minute})
	access, _, err := manager.GenerateAccessTokenWithSessionEpoch(41, "fixture", true, 7, 12)
	if err != nil {
		t.Fatal(err)
	}
	refresh, _, err := manager.GenerateRefreshTokenWithSessionEpoch(41, "test-device", time.Hour, 7, 12)
	if err != nil {
		t.Fatal(err)
	}
	at, err := manager.ParseAccessToken(access)
	if err != nil || at.UserID != 41 || at.Pwe != 7 || !at.MustChangePassword {
		t.Fatal("access token lost existing claims")
	}
	rt, err := manager.ParseRefreshToken(refresh)
	if err != nil || rt.UserID != 41 || rt.Pwe != 7 || rt.DeviceID != "test-device" {
		t.Fatal("refresh token lost existing claims")
	}
	assertSessionEpochClaims(t, at, "db_epoch_v1", "12")
	assertSessionEpochClaims(t, rt, "db_epoch_v1", "12")
	for _, epoch := range []int64{0, -1} {
		if token, id, err := manager.GenerateAccessTokenWithSessionEpoch(41, "fixture", false, 0, epoch); err == nil || token != "" || id != "" {
			t.Errorf("access signing returned a token or ID for invalid epoch %d", epoch)
		}
		if token, id, err := manager.GenerateRefreshTokenWithSessionEpoch(41, "test-device", time.Hour, 0, epoch); err == nil || token != "" || id != "" {
			t.Errorf("refresh signing returned a token or ID for invalid epoch %d", epoch)
		}
	}
}

func TestLegacyTokenSignersOmitDatabaseEpoch(t *testing.T) {
	manager := NewManager(Config{Secret: "synthetic-session-epoch-test-key", AccessTTL: time.Minute})
	access, _, err := manager.GenerateAccessTokenWithEpoch(41, "fixture", false, 5)
	if err != nil {
		t.Fatal(err)
	}
	refresh, _, err := manager.GenerateRefreshToken(41, "test-device", time.Hour, 5)
	if err != nil {
		t.Fatal(err)
	}
	at, err := manager.ParseAccessToken(access)
	if err != nil || at.Pwe != 5 {
		t.Fatal("legacy access token changed")
	}
	rt, err := manager.ParseRefreshToken(refresh)
	if err != nil || rt.Pwe != 5 {
		t.Fatal("legacy refresh token changed")
	}
	assertSessionEpochClaims(t, at, "", "")
	assertSessionEpochClaims(t, rt, "", "")
}

func assertSessionEpochClaims(t *testing.T, claims any, schema, epoch string) {
	t.Helper()
	data, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"session_schema": schema, "session_epoch": epoch} {
		value, present := values[key]
		if want == "" {
			if present {
				t.Errorf("legacy token contains %s", key)
			}
			continue
		}
		var got string
		if err := json.Unmarshal(value, &got); err != nil || got != want {
			t.Errorf("%s = %s, want string %q", key, value, want)
		}
	}
}
