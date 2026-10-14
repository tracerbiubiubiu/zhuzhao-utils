package jsonutil

import (
	"encoding/json"
	"testing"
)

func TestInt64Slice_UnmarshalJSON(t *testing.T) {
	var s Int64Slice
	if err := json.Unmarshal([]byte(`["2","3"]`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 2 || s[0] != 2 || s[1] != 3 {
		t.Fatalf("got %v", s)
	}
	if err := json.Unmarshal([]byte(`[2,3]`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 2 || s[0] != 2 || s[1] != 3 {
		t.Fatalf("got %v", s)
	}
}

func TestInt64Slice_MarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   Int64Slice
		want string
	}{
		{"nil → []（数组恒不 null）", nil, `[]`},
		{"empty → []", Int64Slice{}, `[]`},
		{"元素一律字符串", Int64Slice{1, 2, 3}, `["1","2","3"]`},
		{"大 ID 精度安全", Int64Slice{9223372036854775807}, `["9223372036854775807"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if got := string(b); got != tc.want {
				t.Fatalf("marshal = %s, want %s", got, tc.want)
			}
		})
	}
}

// roundtrip：发送侧字符串 ↔ 收方双形态宽容闭合。
func TestInt64Slice_MarshalUnmarshalRoundtrip(t *testing.T) {
	orig := Int64Slice{1, 9223372036854775807}
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Int64Slice
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back) != len(orig) || back[1] != orig[1] {
		t.Fatalf("roundtrip mismatch: %v", back)
	}
}
