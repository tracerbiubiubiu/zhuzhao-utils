package redis

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

//go:embed scripts/login_lock.lua
var loginLockScript string

// LoginLockOptions 登录失败锁定参数。零值字段取默认值。
type LoginLockOptions struct {
	// KeyPrefix 计数键前缀，实际键 = KeyPrefix + subject。
	// 默认 "lock:login:"。
	KeyPrefix string
	// Window 锁定窗口（首次失败起算，后续失败不重置，防持续爆破刷新窗口）。
	// 默认 15 分钟。
	Window time.Duration
	// MaxFails 允许的失败次数，第 MaxFails+1 次起 blocked。默认 5。
	MaxFails int
}

func defaultLoginLock() LoginLockOptions {
	return LoginLockOptions{
		KeyPrefix: "lock:login:",
		Window:    15 * time.Minute,
		MaxFails:  5,
	}
}

func (o LoginLockOptions) withDefaults() LoginLockOptions {
	d := defaultLoginLock()
	if o.KeyPrefix == "" {
		o.KeyPrefix = d.KeyPrefix
	}
	if o.Window == 0 {
		o.Window = d.Window
	}
	if o.MaxFails == 0 {
		o.MaxFails = d.MaxFails
	}
	return o
}

// Scripts Redis Lua 脚本封装（登录失败锁定等）
type Scripts struct {
	client *goredis.Client
	lock   LoginLockOptions
}

// NewScripts 创建脚本执行器。可传一个 LoginLockOptions 覆盖默认锁定参数。
func NewScripts(client *goredis.Client, opts ...LoginLockOptions) *Scripts {
	lock := defaultLoginLock()
	if len(opts) > 0 {
		lock = opts[len(opts)-1]
	}
	return &Scripts{client: client, lock: lock.withDefaults()}
}

func (s *Scripts) loginLockKey(subject string) string {
	return s.lock.KeyPrefix + subject
}

// LoginLockIsBlocked 校验密码前检查是否已锁定（count > max_fails）
func (s *Scripts) LoginLockIsBlocked(ctx context.Context, subject string) (bool, error) {
	n, err := s.client.Get(ctx, s.loginLockKey(subject)).Int()
	if err == goredis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("login lock get: %w", err)
	}
	return n > s.lock.MaxFails, nil
}

// LoginLockIncr 密码错误时原子 INCR + 首次 EXPIRE；返回 true 表示已超限
func (s *Scripts) LoginLockIncr(ctx context.Context, subject string) (blocked bool, err error) {
	res, err := s.client.Eval(
		ctx,
		loginLockScript,
		[]string{s.loginLockKey(subject)},
		int64(s.lock.Window.Seconds()),
		s.lock.MaxFails,
	).Int()
	if err != nil {
		return false, fmt.Errorf("login lock eval: %w", err)
	}
	return res == 1, nil
}

// LoginLockClear 登录成功后清零计数（锁定恢复入口）
func (s *Scripts) LoginLockClear(ctx context.Context, subject string) error {
	if err := s.client.Del(ctx, s.loginLockKey(subject)).Err(); err != nil {
		return fmt.Errorf("login lock clear: %w", err)
	}
	return nil
}
