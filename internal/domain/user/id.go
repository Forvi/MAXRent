package user

import "strconv"

// ID Идентификатор пользователя в мессенджере. Выдаётся платформой при первом
// обращении к боту, поэтому собственные идентификаторы не генерируются.
type ID int64

// NewID Создаёт идентификатор из значения платформы.
func NewID(raw int64) ID {
	return ID(raw)
}

// String Возвращает строковое представление идентификатора.
func (id ID) String() string {
	return strconv.FormatInt(int64(id), 10)
}

// Int64 Возвращает идентификатор для SQL и API мессенджера.
func (id ID) Int64() int64 {
	return int64(id)
}

// Valid Сообщает, что идентификатор задан.
func (id ID) Valid() bool {
	return id > 0
}
