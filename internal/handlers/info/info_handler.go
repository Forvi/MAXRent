// Package handlers Обработчик команды /info: краткая информация о сервисе.
package info

import (
	"context"
	"log/slog"
	"strings"

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

// HandleUpdate Обрабатывает входящее событие: на /info отвечает описанием сервиса.
// Остальные события игнорируются.
func (h *InfoHandler) HandleUpdate(ctx context.Context, update maxapi.Update) error {
	if update.Type != maxapi.UpdateMessageCreated {
		return nil
	}

	// В группах команда приходит с суффиксом бота: /info@MyBot
	name, _, _ := strings.Cut(update.Command.Name, "@")
	if name != "/info" {
		return nil
	}

	if err := h.sender.SendMessage(ctx, update.ChatID, infoMessage); err != nil {
		h.logger.ErrorContext(
			ctx, "failed to send info",
			"err", err,
			"chat_id", update.ChatID,
		)
		return nil
	}

	h.logger.InfoContext(
		ctx, "command handled",
		"command", name,
		"chat_id", update.ChatID,
		"user_id", update.UserID,
	)

	return nil
}
