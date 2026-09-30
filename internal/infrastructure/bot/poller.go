package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

// Handler Обрабатывает входящее событие бота. Фичи реализуют этот интерфейс,
// чтобы реагировать на апдейты, не зная о транспорте.
//
// Возвращает handled: сообщение обработано этим обработчиком или оно не его.
// Поллер передаёт событие обработчикам по порядку и останавливается на первом
// обработавшем. Без этого на одно сообщение отвечали бы сразу все обработчики,
// и вопросы разных анкет перемешивались бы.
type Handler interface {
	HandleUpdate(ctx context.Context, update maxapi.Update) (handled bool, err error)
}

// Poller Цикл long polling: последовательно запрашивает апдейты и передаёт их обработчикам.
// Сетевые ошибки не завершают работу — цикл ждёт и повторяет запрос, чтобы бот
// переживал кратковременные обрывы связи без перезапуска контейнера.
type Poller struct {
	client *Client
	logger *slog.Logger
	router *Router
}

// NewPoller Создаёт поллер поверх клиента бота.
func NewPoller(client *Client, logger *slog.Logger, handlers ...Handler) *Poller {
	return &Poller{
		client: client,
		logger: logger,
		router: NewRouter(logger, handlers...),
	}
}

// Run выполняет опрос до отмены контекста.
// Возвращает ошибку контекста при штатной остановке, остальное логируется и повторяется.
func (p *Poller) Run(ctx context.Context) error {
	var marker int64

	p.logger.Info("polling started", "handlers", len(p.router.handlers))

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

// dispatch Раздаёт апдейты обработчикам.
func (p *Poller) dispatch(ctx context.Context, updates []maxapi.Update) {
	for _, update := range updates {
		p.router.Route(ctx, update)
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
