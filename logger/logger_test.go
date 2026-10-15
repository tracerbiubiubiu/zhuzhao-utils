// logger 包单测（2026-10-15 清欠批 BK-25）：级别过滤 + JSON Lines 文件落盘。
package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_WritesJSONLinesAndFiltersByLevel(t *testing.T) {
	dir := t.TempDir()
	lg := New(Config{Level: "warn", Dir: dir}) // warn 级：info 应被过滤

	lg.Info("should-not-appear")
	lg.Warn("warn-line", "k", "v")

	// MultiWriter 直写（无异步缓冲），断言前无需 flush
	data, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expect exactly 1 line (info filtered), got %d: %s", len(lines), data)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("line not valid JSON: %v", err)
	}
	if entry["msg"] != "warn-line" || entry["k"] != "v" {
		t.Fatalf("unexpected entry: %v", entry)
	}
}

func TestNew_ZeroConfigDefaultsToInfo(t *testing.T) {
	dir := t.TempDir()
	lg := New(Config{Dir: dir}) // Level 零值 → info（非 debug/info/warn/error 按 info）
	lg.Info("info-line")
	data, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), `"info-line"`) {
		t.Fatalf("zero-level config must pass info logs, got: %s", data)
	}
}
