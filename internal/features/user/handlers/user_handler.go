// Package handlers Обработчики входящих событий (апдейтов) бота для фичи user.
// Это точка расширения под транспорт мессенджера: здесь будет разбор апдейта и вызов юзкейсов.
package handlers

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Forvi/maxrent/internal/features/user/dto"
	"github.com/Forvi/maxrent/internal/features/user/ports"
	"github.com/Forvi/maxrent/internal/features/user/service"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

// helloMessage Ответ на команду /start.
const helloMessage = "Hello"

// UserHandler Обрабатывает события бота, относящиеся к пользователям.
type UserHandler struct {
	createUser *service.CreateUser
	sender     ports.MessageSender
	logger     *slog.Logger
}

// NewUserHandler Создаёт обработчик событий фичи user.
func NewUserHandler(createUser *service.CreateUser, sender ports.MessageSender, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		createUser: createUser,
		sender:     sender,
		logger:     logger,
	}
}

// HandleUpdate Обрабатывает входящее событие бота: разбирает команду и отвечает на неё.
// События, которые фича не обрабатывает, игнорируются без ошибки.
func (h *UserHandler) HandleUpdate(ctx context.Context, update maxapi.Update) error {
	if update.Type != maxapi.UpdateMessageCreated {
		return nil
	}

	h.handleCommand(ctx, update)

	return nil
}

// handleCommand Отвечает на известные команды. Неизвестные команды игнорируются.
func (h *UserHandler) handleCommand(ctx context.Context, update maxapi.Update) {
	// В группах команда приходит с суффиксом бота: /start@MyBot
	name, _, _ := strings.Cut(update.Command.Name, "@")

	if name != "/start" {
		return
	}

	if err := h.sender.SendMessage(ctx, update.ChatID, helloMessage); err != nil {
		h.logger.ErrorContext(ctx, "failed to send greeting",
			"err", err,
			"chat_id", update.ChatID,
		)
		return
	}

	h.logger.InfoContext(ctx, "command handled",
		"command", name,
		"chat_id", update.ChatID,
		"user_id", update.UserID,
	)
}

// Handle Обрабатывает команду регистрации пользователя.
func (h *UserHandler) Handle(ctx context.Context, username string) error {
	user, err := h.createUser.Execute(ctx, dto.CreateUserInput{Username: username})
	if err != nil {
		return err
	}

	h.logger.InfoContext(ctx, "user created", "user_id", user.ID, "username", user.Username)
	return nil
}
