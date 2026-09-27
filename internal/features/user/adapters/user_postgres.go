// Package adapters Реализации портов фичи user на инфраструктуре (PostgreSQL).
package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Forvi/maxrent/internal/features/user/domain"
)

// uniqueViolationCode Код ошибки PostgreSQL для нарушения уникального ограничения.
const uniqueViolationCode = "23505"

// UserRepository Адаптер порта ports.UserWriter поверх database/sql (чистый SQL).
type UserRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewUserRepository Создаёт репозиторий пользователей.
func NewUserRepository(db *sql.DB, logger *slog.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

// CreateUser Вставляет пользователя и возвращает сохранённую сущность одним запросом (INSERT ... RETURNING).
func (r *UserRepository) CreateUser(ctx context.Context, user domain.User) (domain.User, error) {
	const query = `
		INSERT INTO users (id, username, created_at)
		VALUES ($1, $2, $3)
		RETURNING id, username, created_at`

	var saved domain.User
	err := r.db.QueryRowContext(ctx, query, user.ID, user.Username, user.CreatedAt).
		Scan(&saved.ID, &saved.Username, &saved.CreatedAt)
	if err != nil {
		return domain.User{}, r.mapError(ctx, "create user", err)
	}

	return saved, nil
}

// mapError Приводит ошибку БД к доменным ошибкам, логируя неожиданные случаи.
func (r *UserRepository) mapError(ctx context.Context, op string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		return domain.ErrAlreadyExists
	}

	r.logger.ErrorContext(ctx, "database error", "op", op, "err", err)
	return fmt.Errorf("%s: %w", op, err)
}
