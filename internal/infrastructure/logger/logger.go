// Package logger Реализует инициализацию логгера на базе log/slog.
package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
)

// NewLogger Создаёт *slog.Logger: цветной текст для dev, JSON для prod.
// Окружение передаётся явно, чтобы APP_ENV разбирался в одном месте — в config.
func NewLogger(cfg *Config, env string) *slog.Logger {
	level := ParseLevel(cfg.Level)

	if strings.EqualFold(env, "prod") {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		}))
	}

	handler := tint.NewTextHandler(os.Stdout, &tint.Options{
		Level:      level,
		AddSource:  true,
		TimeFormat: "15:04:05.000",
	})

	return slog.New(handler)
}

// ParseLevel Преобразует строковый уровень в slog.Level, по умолчанию INFO.
func ParseLevel(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
