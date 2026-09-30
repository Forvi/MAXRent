// Package user Сценарии работы с пользователями: регистрация и выбор роли.
package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/ports"
)

// Service Сценарии регистрации пользователя и выбора роли в сделке.
type Service struct {
	repo   ports.UserRepository
	logger *slog.Logger
	now    func() time.Time
}

// NewService Создаёт сервис пользователей.
func NewService(repo ports.UserRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// Register Регистрирует пользователя, если он ещё не зарегистрирован.
// Возвращает актуальное состояние: у уже известного пользователя роль сохраняется,
// поэтому повторный /start не сбрасывает выбор.
func (s *Service) Register(ctx context.Context, id domainuser.ID) (domainuser.User, error) {
	if !id.Valid() {
		return domainuser.User{}, fmt.Errorf("invalid user id: %d", id.Int64())
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domainuser.ErrNotFound) {
		return domainuser.User{}, fmt.Errorf("lookup user: %w", err)
	}

	created := domainuser.New(id, s.now())
	if err := s.repo.Create(ctx, created); err != nil {
		return domainuser.User{}, fmt.Errorf("register user: %w", err)
	}

	s.logger.InfoContext(ctx, "user registered", "user_id", id.String())

	return created, nil
}

// SetRole Сохраняет роль пользователя.
// Выбор роли приходит нажатием кнопки, payload разбирает обработчик:
// сервис работает с готовой ролью и о кнопках не знает.
func (s *Service) SetRole(
	ctx context.Context,
	id domainuser.ID,
	role domainuser.Role,
) (domainuser.User, error) {
	if !role.Valid() {
		return domainuser.User{}, fmt.Errorf("%w: %q", domainuser.ErrUnknownRole, role)
	}

	registered, err := s.Register(ctx, id)
	if err != nil {
		return domainuser.User{}, err
	}

	if err := s.repo.SetRole(ctx, id, role); err != nil {
		return domainuser.User{}, fmt.Errorf("save role: %w", err)
	}

	s.logger.InfoContext(ctx, "user role set",
		"user_id", id.String(),
		"role", role,
		"previous_role", registered.RoleTitle(),
	)

	return registered.WithRole(role), nil
}

// Get возвращает зарегистрированного пользователя с его ролью.
func (s *Service) Get(ctx context.Context, id domainuser.ID) (domainuser.User, error) {
	found, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return domainuser.User{}, fmt.Errorf("get user: %w", err)
	}

	return found, nil
}
