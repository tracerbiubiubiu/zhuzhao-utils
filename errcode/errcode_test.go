// errcode 包单测（2026-10-15 清欠批 BK-25）：构造器语义 + 通用段哨兵完整性。
package errcode

import "testing"

func TestNew(t *testing.T) {
	e := New(10442, "测试错误")
	if e.Code != 10442 || e.Message != "测试错误" {
		t.Fatalf("New must keep code/message, got %+v", e)
	}
}

// TestUniversalSentinels 通用段哨兵：码值唯一、落在 10000-10999、文案非空——
// 跨仓引用 these 的对账前提（zhuzhao internal errcode 亦转发此段）。
func TestUniversalSentinels(t *testing.T) {
	all := map[int]string{
		ErrInternal.Code:           ErrInternal.Message,
		ErrInvalidParams.Code:      ErrInvalidParams.Message,
		ErrUnauthorized.Code:       ErrUnauthorized.Message,
		ErrTooManyReqs.Code:        ErrTooManyReqs.Message,
		ErrServiceUnavailable.Code: ErrServiceUnavailable.Message,
	}
	seen := map[int]bool{}
	for code, msg := range all {
		if code < 10000 || code > 10999 {
			t.Fatalf("code %d outside universal segment 10000-10999", code)
		}
		if msg == "" {
			t.Fatalf("code %d empty message", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %d", code)
		}
		seen[code] = true
	}
}
