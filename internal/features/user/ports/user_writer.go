// Package ports Описывает порты (интерфейсы), которые реализуют адаптеры.
// Является контрактом между ядром приложения и внешним миром.
package ports

import (
	"context"

	"github.com/Forvi/maxrent/internal/features/user/domain"
)

// UserWriter Порт записи пользователей. Реализуется адаптером в adapters.
//
//go:generate mockery --name UserWriter --output ./mocks
type UserWriter interface {
	// CreateUser Создаёт пользователя и возвращает сохранённую сущность.
	CreateUser(ctx context.Context, user domain.User) (domain.User, error)
}
