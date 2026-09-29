// Package app Точка сборки приложения: конфигурация, зависимости и жизненный цикл.
package app

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/Forvi/maxrent/internal/infrastructure/bot"
	"github.com/Forvi/maxrent/internal/infrastructure/config"
)

// App Хранит всё, что нужно приложению для работы.
type App struct {
	cfg             *config.Config
	db              *sql.DB
	logger          *slog.Logger
	poller          *bot.Poller
	shutdownTimeout time.Duration
}
