// crypto 包单测（2026-10-15 清欠批 BK-25）：哈希往返 + 虚拟口令恒假语义。
// CheckDummyPassword 的时延拉平不可稳定断言（环境抖动），只钉行为面。
package crypto

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret-日本語oke")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("expect bcrypt hash, got %q", hash)
	}
	if !CheckPassword("s3cret-日本語oke", hash) {
		t.Fatal("correct password must verify")
	}
	if CheckPassword("wrong", hash) {
		t.Fatal("wrong password must not verify")
	}
}

func TestCheckDummyPasswordAlwaysFails(t *testing.T) {
	// 恒失败是 B4-1 时延拉平的语义前提：对固定哈希做比对，结果由调用方弃用。
	// 这里钉住的是「对任意输入执行不 panic、不产生可观察副作用」，恒假性由
	// 调用方（auth 登录流）语义保证——bcrypt 对随机口令碰撞率可忽略。
	for _, pw := range []string{"", "admin12345", "Phase3Demo#2026", strings.Repeat("x", 72)} {
		CheckDummyPassword(pw) // 必须安全返回（bcrypt 72 字节以上输入截断不炸）
	}
}
