// Package handlers Обработчик команды /info: краткая информация о сервисе.
package info

import (
	"context"
	"log/slog"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports"
)

// infoMessage Текст ответа на команду /info.
const infoMessage = `MAXRent — помощник по оформлению договора аренды жилья.

Помогает наймодателю и нанимателю составить договор найма и акт приёма-передачи
прямо в чате: пошаговая анкета, проверка полноты условий, готовые документы.

Команды:
/start — начать оформление
/info — описание сервиса`

// InfoHandler Отвечает на команду /info.
type InfoHandler struct {
	sender ports.MessageSender
	logger *slog.Logger
}

// NewInfoHandler Создаёт обработчик команды /info.
func NewInfoHandler(sender ports.MessageSender, logger *slog.Logger) *InfoHandler {
	return &InfoHandler{
		sender: sender,
		logger: logger,
	}
}

// HandleUpdate Отвечает на /info описанием сервиса.
// Второе значение: true, если событие обработано здесь. Остальные события
// оставляются другим обработчикам.
func (h *InfoHandler) HandleUpdate(ctx context.Context, update maxapi.Update) (bool, error) {
	if update.Type != maxapi.UpdateMessageCreated {
		return false, nil
	}

	if update.CommandName() != "/info" {
		return false, nil
	}

	if err := h.sender.SendMessage(ctx, update.ChatID, infoMessage); err != nil {
		h.logger.ErrorContext(
			ctx, "failed to send info",
			"err", err,
			"chat_id", update.ChatID,
		)

		return true, nil
	}

	h.logger.InfoContext(
		ctx, "command handled",
		"command", update.CommandName(),
		"chat_id", update.ChatID,
		"user_id", update.UserID,
	)

	return true, nil
}
