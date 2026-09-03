package jsonutil

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Int64Slice 支持 JSON 数组元素为 string 或 number（API 文档约定 ID 用字符串）
type Int64Slice []int64

func (s *Int64Slice) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make([]int64, 0, len(raw))
	for _, item := range raw {
		var n int64
		if err := json.Unmarshal(item, &n); err == nil {
			out = append(out, n)
			continue
		}
		var str string
		if err := json.Unmarshal(item, &str); err != nil {
			return fmt.Errorf("invalid int64 slice element: %w", err)
		}
		v, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int64 string %q: %w", str, err)
		}
		out = append(out, v)
	}
	*s = out
	return nil
}
