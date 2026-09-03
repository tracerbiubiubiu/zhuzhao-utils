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
