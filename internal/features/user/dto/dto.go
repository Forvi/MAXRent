// Package dto Структуры входных и выходных данных юзкейсов фичи user.
package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateUserInput Входные данные юзкейса создания пользователя.
type CreateUserInput struct {
	Username string
}

// UserDTO Представление пользователя на выходе юзкейсов.
type UserDTO struct {
	ID        uuid.UUID
	Username  string
	CreatedAt time.Time
}
