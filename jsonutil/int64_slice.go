package jsonutil

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Int64Slice 支持 JSON 数组元素为 string 或 number（API 文档约定 ID 用字符串）
type Int64Slice []int64

// MarshalJSON 发送侧序列化：元素一律字符串（standards §3-12「发送侧一律字符串」；
// 2026-10-14 契约审计补齐——原实现仅收方宽容，出站仍为裸数字数组，大 ID 经 JS
// Number 有精度丢失风险）。nil 与空切片均序列化为 []（数组恒不 null——本类型
// 的职责即归一化 ID 数组线格式；注意与原生 []int64 nil→null 语义不同）。
func (s Int64Slice) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, len(s)*10+2)
	buf = append(buf, '[')
	for i, v := range s {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = strconv.AppendInt(buf, v, 10)
		buf = append(buf, '"')
	}
	return append(buf, ']'), nil
}

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
			return fmt.Errorf("jsonutil: invalid int64 slice element: %w", err)
		}
		v, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return fmt.Errorf("jsonutil: invalid int64 string %q: %w", str, err)
		}
		out = append(out, v)
	}
	*s = out
	return nil
}
