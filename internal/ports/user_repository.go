package ports

import (
	"context"

	"github.com/Forvi/maxrent/internal/domain/user"
)

// UserRepository Порт хранения пользователей. Реализуется адаптером в repositories.
type UserRepository interface {
	// Create Регистрирует пользователя. Повторная регистрация того же id не изменяет запись.
	Create(ctx context.Context, u user.User) error
	// FindByID Возвращает пользователя. Если пользователя нет — user.ErrNotFound.
	FindByID(ctx context.Context, id user.ID) (user.User, error)
	// SetRole Сохраняет выбранную роль пользователя.
	SetRole(ctx context.Context, id user.ID, role user.Role) error
}
