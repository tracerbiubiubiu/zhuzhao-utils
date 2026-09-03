package postgres

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config PostgreSQL 连接配置。零值字段由 ApplyDefaults 补安全默认。
type Config struct {
	Host             string
	Port             int
	User             string
	Password         string
	DBName           string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	ConnMaxIdleTime  time.Duration
	ConnectTimeout   time.Duration
	SSLMode          string
	StatementTimeout time.Duration // 0 表示不设置
	ApplicationName  string        // 空 = 不设置该运行时参数
}

// ApplyDefaults 补零值默认。应用名不带默认值（库不该替调用方起名），
// 需要时由调用方显式设置。
func (c *Config) ApplyDefaults() {
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 25
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = time.Hour
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 30 * time.Minute
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 5 * time.Second
	}
	if c.SSLMode == "" {
		c.SSLMode = "disable"
	}
}

// DSN 返回 PostgreSQL 连接字符串。
// 用 net/url 构造，UserPassword 对密码中的保留字符（@ : / ? # % 等）
// 自动转义——密码常经环境变量注入，字符不受控。
func (c Config) DSN() string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     c.DBName,
		RawQuery: "sslmode=" + url.QueryEscape(c.SSLMode),
	}
	return u.String()
}

// New 创建 PostgreSQL 连接池
func New(cfg Config) (*pgxpool.Pool, func(), error) {
	cfg.ApplyDefaults()

	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	// describe 结果按 SQL 文本缓存（按连接隔离）——无显式 Prepare 的场景下，
	// 默认模式每查询重复 parse/describe/plan；一行切换全部 repository 受益。
	// 在线 DDL、PREPARE 重名场景不适用。
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe

	if poolConfig.ConnConfig.RuntimeParams == nil {
		poolConfig.ConnConfig.RuntimeParams = make(map[string]string)
	}
	if cfg.ApplicationName != "" {
		poolConfig.ConnConfig.RuntimeParams["application_name"] = cfg.ApplicationName
	}
	if cfg.StatementTimeout > 0 {
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Ping 独立超时——若与建池共用 ConnectTimeout 的 ctx，DNS+TLS 慢时
	// 建池已耗尽预算，Ping 必超时导致初始化误判（池本身可能可用）
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	cleanup := func() {
		pool.Close()
	}
	return pool, cleanup, nil
}
