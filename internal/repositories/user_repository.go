// Package repositories Реализации портов хранения на PostgreSQL.
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Forvi/maxrent/internal/domain/user"
)

// UserRepositoryAdapter Хранение пользователей в PostgreSQL.
type UserRepositoryAdapter struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewUserRepositoryAdapter Создаёт адаптер хранилища пользователей.
func NewUserRepositoryAdapter(db *sql.DB, logger *slog.Logger) *UserRepositoryAdapter {
	return &UserRepositoryAdapter{
		db:     db,
		logger: logger,
	}
}

// Create Регистрирует пользователя. Повторный вызов с тем же id ничего не меняет:
// ON CONFLICT DO NOTHING сохраняет уже выбранную роль.
func (r *UserRepositoryAdapter) Create(ctx context.Context, u user.User) error {
	const query = `
		INSERT INTO users (id, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (id) DO NOTHING`

	if _, err := r.db.ExecContext(ctx, query, u.ID.Int64(), roleToDB(u.Role), u.CreatedAt); err != nil {
		r.logger.ErrorContext(ctx, "failed to create user", "err", err, "user_id", u.ID.String())

		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

// FindByID Возвращает пользователя по идентификатору.
func (r *UserRepositoryAdapter) FindByID(ctx context.Context, id user.ID) (user.User, error) {
	const query = `SELECT id, role, created_at FROM users WHERE id = $1`

	var (
		rowID     int64
		role      sql.NullString
		createdAt time.Time
	)

	err := r.db.QueryRowContext(ctx, query, id.Int64()).Scan(&rowID, &role, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, user.ErrNotFound
		}

		r.logger.ErrorContext(ctx, "failed to find user", "err", err, "user_id", id.String())

		return user.User{}, fmt.Errorf("find user by id: %w", err)
	}

	found := user.New(user.NewID(rowID), createdAt)
	if role.Valid {
		parsed, err := user.NewRole(role.String)
		if err != nil {
			r.logger.ErrorContext(ctx, "stored role is unknown", "err", err, "user_id", id.String())

			return user.User{}, err
		}

		found = found.WithRole(parsed)
	}

	return found, nil
}

// SetRole Сохраняет выбранную роль пользователя.
func (r *UserRepositoryAdapter) SetRole(ctx context.Context, id user.ID, role user.Role) error {
	const query = `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`

	res, err := r.db.ExecContext(ctx, query, string(role), id.Int64())
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to set role", "err", err, "user_id", id.String())

		return fmt.Errorf("set role: %w", err)
	}

	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return user.ErrNotFound
	}

	return nil
}

// roleToDB готовит роль к записи: nil — роль не выбрана.
func roleToDB(role *user.Role) any {
	if role == nil {
		return nil
	}

	return string(*role)
}
