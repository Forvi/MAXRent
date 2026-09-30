package bot

import (
	"context"
	"log/slog"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

// Router Распределяет события между обработчиками.
//
// Правило одно: событие получает первый обработчик, который признал его своим.
// Это обязательное условие — обработчиков несколько, и все они умеют вести
// диалог. Без остановки на первом обработавшем одно сообщение вызывало бы
// несколько вопросов сразу: адрес из анкеты заявки уходил бы ещё и в анкету
// договора, а вопросы перемешивались бы.
type Router struct {
	handlers []Handler
	logger   *slog.Logger
}

// NewRouter Создаёт маршрутизатор для заданного порядка обработчиков.
func NewRouter(logger *slog.Logger, handlers ...Handler) *Router {
	return &Router{
		handlers: handlers,
		logger:   logger,
	}
}

// Route передаёт событие первому обработавшему его обработчику.
//
// Если событие не признал никто, оно игнорируется: так бот не отвечает на
// сообщения, которых не понимает. Ошибка отдельного обработчика логируется и
// не прерывает обработку — иначе одно сбойное событие заблокировало бы очередь.
func (r *Router) Route(ctx context.Context, update maxapi.Update) {
	for _, handler := range r.handlers {
		handled, err := handler.HandleUpdate(ctx, update)
		if err != nil {
			r.logger.ErrorContext(ctx, "handle update failed",
				"err", err,
				"update_type", update.Type,
				"chat_id", update.ChatID,
			)
		}

		if handled {
			return
		}
	}
}
