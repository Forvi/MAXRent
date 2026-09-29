package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

// Handler Обрабатывает входящее событие бота. Фичи реализуют этот интерфейс,
// чтобы реагировать на апдейты, не зная о транспорте.
type Handler interface {
	HandleUpdate(ctx context.Context, update maxapi.Update) error
}

// Poller Цикл long polling: последовательно запрашивает апдейты и передаёт их обработчикам.
// Сетевые ошибки не завершают работу — цикл ждёт и повторяет запрос, чтобы бот
// переживал кратковременные обрывы связи без перезапуска контейнера.
type Poller struct {
	client   *Client
	logger   *slog.Logger
	handlers []Handler
}

// NewPoller Создаёт поллер поверх клиента бота.
func NewPoller(client *Client, logger *slog.Logger, handlers ...Handler) *Poller {
	return &Poller{
		client:   client,
		logger:   logger,
		handlers: handlers,
	}
}

// Run выполняет опрос до отмены контекста.
// Возвращает ошибку контекста при штатной остановке, остальное логируется и повторяется.
func (p *Poller) Run(ctx context.Context) error {
	var marker int64

	p.logger.Info("polling started", "handlers", len(p.handlers))

	for {
		if err := ctx.Err(); err != nil {
			p.logger.Info("polling stopped")
			return err
		}

		updates, next, err := p.client.GetUpdates(ctx, marker)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				p.logger.Info("polling stopped")
				return ctxErr
			}

			p.logger.Error("get updates failed, retrying", "err", err, "retry_in", p.client.errorRetryPause())
			if !sleep(ctx, p.client.errorRetryPause()) {
				p.logger.Info("polling stopped")
				return ctx.Err()
			}
			continue
		}

		marker = next
		p.dispatch(ctx, updates)
	}
}

// dispatch Передаёт пачку апдейтов обработчикам. Ошибка отдельного обработчика
// не прерывает обработку остальных.
func (p *Poller) dispatch(ctx context.Context, updates []maxapi.Update) {
	for _, update := range updates {
		for _, handler := range p.handlers {
			if err := handler.HandleUpdate(ctx, update); err != nil {
				p.logger.ErrorContext(ctx, "handle update failed",
					"err", err,
					"update_type", update.Type,
					"chat_id", update.ChatID,
				)
			}
		}
	}
}

// sleep Ждёт d, возвращает false, если контекст отменён.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
