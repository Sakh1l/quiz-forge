package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/quizforge/quiz-forge/internal/config"
)

type Logger struct {
	*slog.Logger
	cfg *config.Config
}

var defaultLogger *Logger

func Init(cfg *config.Config) *Logger {
	var handler slog.Handler
	var output io.Writer = os.Stdout

	level := parseLevel(cfg.LogLevel)

	if cfg.IsDevelopment() {
		handler = newDevHandler(output, level)
	} else {
		handler = newProdHandler(output, level)
	}

	logger := &Logger{
		Logger: slog.New(handler),
		cfg:    cfg,
	}

	defaultLogger = logger
	slog.SetDefault(logger.Logger)

	return logger
}

func Get() *Logger {
	if defaultLogger == nil {
		panic("logger not initialized, call Init() first")
	}
	return defaultLogger
}

func parseLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newDevHandler(output io.Writer, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}
	return slog.NewTextHandler(output, opts)
}

func newProdHandler(output io.Writer, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339))
			}
			return a
		},
	}
	return slog.NewJSONHandler(output, opts)
}

func (l *Logger) IsDebugEnabled() bool {
	return l.cfg.LogLevel == "debug"
}

type contextKey string

const (
	nicknameKey contextKey = "nickname"
)

func WithNickname(ctx context.Context, nickname string) context.Context {
	return context.WithValue(ctx, nicknameKey, nickname)
}

func GetNicknameFromContext(ctx context.Context) string {
	if v := ctx.Value(nicknameKey); v != nil {
		if nickname, ok := v.(string); ok {
			return nickname
		}
	}
	return ""
}
