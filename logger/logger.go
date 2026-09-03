package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置。零值字段的行为：Level 非 debug/info/warn/error 一律按 info；
// MaxSize/MaxBackups/MaxAge 为 0 时走 lumberjack 内置默认（100MB / 0 不限 / 0 不限）。
type Config struct {
	Level      string // debug | info | warn | error
	Dir        string // 日志目录，写入 Dir/app.log
	MaxSize    int    // 单文件上限 MB
	MaxBackups int    // 保留的历史文件数
	MaxAge     int    // 保留天数
}

// New 创建 slog Logger，同时输出到文件和控制台
func New(cfg Config) *slog.Logger {
	var slogLevel slog.Level
	switch cfg.Level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	writer := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Dir, "app.log"),
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   true,
	}

	multiWriter := io.MultiWriter(writer, os.Stdout)

	handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		Level:     slogLevel,
		AddSource: true,
	})

	return slog.New(handler)
}
