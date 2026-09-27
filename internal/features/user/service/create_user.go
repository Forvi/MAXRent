// Package service Содержит юзкейсы (use cases) фичи user.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Forvi/maxrent/internal/features/user/domain"
	"github.com/Forvi/maxrent/internal/features/user/dto"
	"github.com/Forvi/maxrent/internal/features/user/ports"
)

// ErrInternal Ошибка внутренняя, не связанная с бизнес-правилами.
// Доменные ошибки пробрасываются как есть, всё остальное оборачивается сюда,
// чтобы вызывающая сторона могла отличить одно от другого через errors.Is.
var ErrInternal = errors.New("internal error")

// CreateUser Юзкейс регистрации пользователя.
type CreateUser struct {
	writer ports.UserWriter
	logger *slog.Logger
}

// NewCreateUser Создаёт юзкейс создания пользователя.
func NewCreateUser(writer ports.UserWriter, logger *slog.Logger) *CreateUser {
	return &CreateUser{
		writer: writer,
		logger: logger,
	}
}

// Execute Валидирует входные данные и сохраняет нового пользователя.
func (s *CreateUser) Execute(ctx context.Context, input dto.CreateUserInput) (dto.UserDTO, error) {
	user, err := domain.NewUser(input.Username)
	if err != nil {
		return dto.UserDTO{}, fmt.Errorf("validate user: %w", err)
	}

	saved, err := s.writer.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) || errors.Is(err, domain.ErrNotFound) {
			return dto.UserDTO{}, err
		}
		s.logger.Error("failed to create user", "err", err, "user_id", user.ID)
		return dto.UserDTO{}, fmt.Errorf("%w: create user: %w", ErrInternal, err)
	}

	return dto.UserDTO{
		ID:        saved.ID,
		Username:  saved.Username,
		CreatedAt: saved.CreatedAt,
	}, nil
}
