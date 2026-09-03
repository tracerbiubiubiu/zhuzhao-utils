package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config Redis 连接配置。零值字段由 ApplyDefaults 补安全默认。
type Config struct {
	Host            string
	Port            int
	DB              int
	Password        string
	PoolSize        int
	MinIdleConns    int
	PoolTimeout     time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxRetries      int
	ConnMaxIdleTime time.Duration
}

// ApplyDefaults 补零值默认
func (c *Config) ApplyDefaults() {
	if c.PoolSize == 0 {
		c.PoolSize = 20
	}
	if c.MinIdleConns == 0 {
		c.MinIdleConns = 5
	}
	if c.PoolTimeout == 0 {
		c.PoolTimeout = 4 * time.Second
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 5 * time.Second
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 3 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 3 * time.Second
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 2
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 5 * time.Minute
	}
}

// Addr 返回 Redis 地址
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// New 创建 Redis 客户端
func New(cfg Config) (*redis.Client, func(), error) {
	cfg.ApplyDefaults()

	client := redis.NewClient(&redis.Options{
		Addr:            cfg.Addr(),
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		PoolTimeout:     cfg.PoolTimeout,
		DialTimeout:     cfg.DialTimeout,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		MaxRetries:      cfg.MaxRetries,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	cleanup := func() {
		client.Close()
	}
	return client, cleanup, nil
}
