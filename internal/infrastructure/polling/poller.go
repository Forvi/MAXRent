// Package polling Реализация цикла опроса апдейтов (long polling) мессенджера.
package polling

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// Poller Циклически опрашивает источник апдейтов и проверяет доступность инфраструктуры.
// Временная заглушка: будет заменена на клиент мессенджера.
type Poller struct {
	interval time.Duration
	db       *sql.DB
	logger   *slog.Logger
}

// NewPoller Создаёт поллер с заданным интервалом опроса.
func NewPoller(interval time.Duration, db *sql.DB, logger *slog.Logger) *Poller {
	return &Poller{
		interval: interval,
		db:       db,
		logger:   logger,
	}
}

// Run выполняет опрос до отмены контекста.
func (p *Poller) Run(ctx context.Context) error {
	p.logger.Info("polling started", "interval", p.interval)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("polling stopped")
			return ctx.Err()
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

// tick Один цикл опроса: проверка доступности БД.
// После подключения клиента мессенджера здесь будет запрос апдейтов и их передача обработчикам фич.
func (p *Poller) tick(ctx context.Context) {
	if err := p.db.PingContext(ctx); err != nil {
		p.logger.WarnContext(ctx, "database is unavailable", "err", err)
	}
}
