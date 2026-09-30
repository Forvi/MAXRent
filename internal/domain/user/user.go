// Package user Домен пользователя: идентичность в мессенджере и роль в сделке.
package user

import (
	"errors"
	"time"
)

// ErrNotFound Пользователь не найден.
var ErrNotFound = errors.New("user not found")

// User Пользователь бота. Роль выбирается один раз при регистрации,
// поэтому хранится указателем: nil означает «роль ещё не выбрана».
type User struct {
	// ID идентификатор пользователя в мессенджере.
	ID ID
	// Role роль в сделке, nil пока не выбрана.
	Role *Role
	// CreatedAt момент регистрации.
	CreatedAt time.Time
}

// New Создаёт зарегистрированного пользователя без роли.
func New(id ID, createdAt time.Time) User {
	return User{
		ID:        id,
		CreatedAt: createdAt,
	}
}

// WithRole Возвращает копию пользователя с указанной ролью.
func (u User) WithRole(role Role) User {
	u.Role = &role

	return u
}

// HasRole Сообщает, выбрана ли роль.
func (u User) HasRole() bool {
	return u.Role != nil
}

// RoleTitle Возвращает роль для показа пользователю или пустую строку,
// если роль не выбрана.
func (u User) RoleTitle() string {
	if u.Role == nil {
		return ""
	}

	return u.Role.Title()
}
