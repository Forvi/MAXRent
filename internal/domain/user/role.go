package user

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnknownRole Получена неизвестная роль.
var ErrUnknownRole = errors.New("unknown role")

// Role Роль пользователя в сделке аренды жилья.
type Role string

// Роли пользователя.
const (
	// RoleTenant Арендатель — наниматель жилого помещения.
	RoleTenant Role = "tenant"
	// RoleLandlord Арендодатель — собственник, сдающий жильё.
	RoleLandlord Role = "landlord"
)

// Payload кнопок выбора роли: значение приходит боту при нажатии.
const (
	// PayloadTenant payload кнопки «Арендатор».
	PayloadTenant = "role_tenant"
	// PayloadLandlord payload кнопки «Арендодатель».
	PayloadLandlord = "role_landlord"
)

// NewRole Разбирает строковое значение роли, как оно хранится в БД.
func NewRole(value string) (Role, error) {
	role := Role(strings.TrimSpace(value))
	if !role.Valid() {
		return "", fmt.Errorf("%w: %q", ErrUnknownRole, value)
	}

	return role, nil
}

// RoleFromPayload Разбирает payload нажатой кнопки в роль.
// Это забота транспорта: сервис работает с готовой Role и о payload не знает.
func RoleFromPayload(payload string) (Role, error) {
	switch Role(strings.TrimSpace(payload)) {
	case RoleTenant, PayloadTenant:
		return RoleTenant, nil
	case RoleLandlord, PayloadLandlord:
		return RoleLandlord, nil
	default:
		return "", fmt.Errorf("%w: payload %q", ErrUnknownRole, payload)
	}
}

// Title Возвращает роль в виде, пригодном для показа пользователю.
func (r Role) Title() string {
	switch r {
	case RoleTenant:
		return "Арендатор"
	case RoleLandlord:
		return "Арендодатель"
	default:
		return string(r)
	}
}

// Valid Сообщает, что роль относится к известным.
func (r Role) Valid() bool {
	return r == RoleTenant || r == RoleLandlord
}
