// Package domain Доменные сущности, правила и ошибки фичи user.
package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User Доменная сущность пользователя.
type User struct {
	ID        uuid.UUID
	Username  string
	CreatedAt time.Time
}

// NewUser Создаёт валидного пользователя с новым идентификатором.
func NewUser(username string) (User, error) {
	name, err := NewUsername(username)
	if err != nil {
		return User{}, err
	}

	return User{
		ID:        uuid.New(),
		Username:  name.Val(),
		CreatedAt: time.Now().UTC(),
	}, nil
}

// Username Value-object с проверкой длины и допустимых символов.
type Username string

// ErrUsernameLength Нарушена длина имени пользователя.
var ErrUsernameLength = errors.New("username must be between 3 and 32 characters")

// NewUsername Валидирует и создаёт Username.
func NewUsername(raw string) (Username, error) {
	name := Username(strings.TrimSpace(raw))
	if len([]rune(name)) < 3 || len([]rune(name)) > 32 {
		return "", ErrUsernameLength
	}
	return name, nil
}

// Val Возвращает строковое значение объекта.
func (u Username) Val() string {
	return string(u)
}

// ErrNotFound Пользователь не найден.
var ErrNotFound = errors.New("user not found")

// ErrAlreadyExists Пользователь с таким именем уже существует.
var ErrAlreadyExists = errors.New("user already exists")
