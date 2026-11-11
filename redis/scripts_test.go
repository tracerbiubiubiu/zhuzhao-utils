package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newTestScripts(t *testing.T) (*Scripts, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewScripts(client), mr
}

// 阈值语义——第 1~5 次失败不锁定（返回 0），第 6 次起返回 1（blocked）。
// 阈值边界（=5 与 >5）必须有测试覆盖。
func TestLoginLockIncr_Threshold(t *testing.T) {
	s, _ := newTestScripts(t)
	ctx := context.Background()
	maxFails := defaultLoginLock().MaxFails

	for i := 1; i <= maxFails; i++ {
		blocked, err := s.LoginLockIncr(ctx, "u-900001")
		if err != nil {
			t.Fatalf("incr #%d: %v", i, err)
		}
		if blocked {
			t.Fatalf("第 %d 次失败（≤ 阈值 %d）不应锁定", i, maxFails)
		}
	}
	// 第 6 次（n=6 > 5）→ blocked
	blocked, err := s.LoginLockIncr(ctx, "u-900001")
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("超过阈值后应返回 blocked=true")
	}
	// IsBlocked 视图一致（count > max_fails）
	is, err := s.LoginLockIsBlocked(ctx, "u-900001")
	if err != nil || !is {
		t.Fatalf("IsBlocked = %v, %v；want true, nil", is, err)
	}
}

// EXPIRE 时机——仅首次 INCR 设置 TTL（窗口 15min），
// 后续失败不重置窗口（防「持续爆破刷新窗口」绕过）。
func TestLoginLockIncr_ExpireOnlyOnFirst(t *testing.T) {
	s, mr := newTestScripts(t)
	ctx := context.Background()
	const subject = "u-900002"
	window := defaultLoginLock().Window

	if _, err := s.LoginLockIncr(ctx, subject); err != nil {
		t.Fatal(err)
	}
	ttl1 := mr.TTL(defaultLoginLock().KeyPrefix + subject)
	if ttl1 <= 0 {
		t.Fatalf("首次失败后应有 TTL，实际 %v", ttl1)
	}
	if ttl1 > window {
		t.Fatalf("TTL %v 超过窗口 %v", ttl1, window)
	}

	// 快进接近窗口末尾后再失败一次——若错误地重置 EXPIRE，TTL 会回到完整窗口
	mr.FastForward(window - 2*time.Second)
	if _, err := s.LoginLockIncr(ctx, subject); err != nil {
		t.Fatal(err)
	}
	ttl2 := mr.TTL(defaultLoginLock().KeyPrefix + subject)
	if ttl2 > 3*time.Second {
		t.Fatalf("后续失败不应重置窗口：TTL=%v（应接近 2s）", ttl2)
	}
}

// 登录成功清零计数（锁定恢复入口）
func TestLoginLockClear(t *testing.T) {
	s, _ := newTestScripts(t)
	ctx := context.Background()
	const subject = "u-900003"
	maxFails := defaultLoginLock().MaxFails

	for i := 0; i < maxFails+1; i++ {
		_, _ = s.LoginLockIncr(ctx, subject)
	}
	if is, _ := s.LoginLockIsBlocked(ctx, subject); !is {
		t.Fatal("前置：应处于锁定态")
	}
	if err := s.LoginLockClear(ctx, subject); err != nil {
		t.Fatal(err)
	}
	if is, err := s.LoginLockIsBlocked(ctx, subject); err != nil || is {
		t.Fatalf("清零后 IsBlocked = %v, %v；want false, nil", is, err)
	}
}

// 不同主体计数隔离（锁定不殃及他人）
func TestLoginLock_IsolationBySubject(t *testing.T) {
	s, _ := newTestScripts(t)
	ctx := context.Background()
	maxFails := defaultLoginLock().MaxFails

	for i := 0; i < maxFails+1; i++ {
		_, _ = s.LoginLockIncr(ctx, "u-900004")
	}
	if is, _ := s.LoginLockIsBlocked(ctx, "u-900005"); is {
		t.Fatal("不同主体不应被连带锁定")
	}
}

// 自定义参数生效：前缀 / 阈值
func TestLoginLock_CustomOptions(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	s := NewScripts(client, LoginLockOptions{KeyPrefix: "guard:", MaxFails: 2})
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		blocked, err := s.LoginLockIncr(ctx, "u-1")
		if err != nil {
			t.Fatal(err)
		}
		if blocked {
			t.Fatal("未到自定义阈值不应锁定")
		}
	}
	if blocked, err := s.LoginLockIncr(ctx, "u-1"); err != nil || !blocked {
		t.Fatalf("超过自定义阈值应锁定，got %v, %v", blocked, err)
	}
	if _, err := client.Get(ctx, "guard:u-1").Result(); err != nil {
		t.Fatalf("自定义前缀未生效: %v", err)
	}
}

// wrapNilHook 模拟埋点/追踪类 hook 对命令错误做 %w 包装（go-redis 的 hook 机制允许）。
type wrapNilHook struct{}

func (wrapNilHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }
func (wrapNilHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		if err := next(ctx, cmd); err != nil {
			wrapped := fmt.Errorf("traced: %w", err)
			cmd.SetErr(wrapped)
			return wrapped
		}
		return nil
	}
}
func (wrapNilHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}

// 2026-11-09：缺键须按 errors.Is 判定——接入包装错误的 hook 后，== goredis.Nil 会把
// 「从未失败过的账号」误报为 Redis 故障（登录链路直接 500/503）。
func TestLoginLockIsBlocked_WrappedNil(t *testing.T) {
	s, _ := newTestScripts(t)
	s.client.AddHook(wrapNilHook{})
	is, err := s.LoginLockIsBlocked(context.Background(), "u-900099")
	if err != nil || is {
		t.Fatalf("缺键（包装后的 redis.Nil）应视为未锁定：got %v, %v；want false, nil", is, err)
	}
}
