// Package handlers Обработчики входящих событий (апдейтов) бота для фичи user.
// Это точка расширения под транспорт мессенджера: здесь будет разбор апдейта и вызов юзкейсов.
package handlers

import (
	"context"
	"log/slog"

	"github.com/Forvi/maxrent/internal/features/user/dto"
	"github.com/Forvi/maxrent/internal/features/user/service"
)

// UserHandler Обрабатывает события бота, относящиеся к пользователям.
type UserHandler struct {
	createUser *service.CreateUser
	logger     *slog.Logger
}

// NewUserHandler Создаёт обработчик событий фичи user.
func NewUserHandler(createUser *service.CreateUser, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		createUser: createUser,
		logger:     logger,
	}
}

// Handle Обрабатывает команду регистрации пользователя.
// Пример временной заглушки: будет заменён на разбор апдейта MAX.
func (h *UserHandler) Handle(ctx context.Context, username string) error {
	user, err := h.createUser.Execute(ctx, dto.CreateUserInput{Username: username})
	if err != nil {
		return err
	}

	h.logger.InfoContext(ctx, "user created", "user_id", user.ID, "username", user.Username)
	return nil
}
