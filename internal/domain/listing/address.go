package listing

import (
	"fmt"
	"strings"
	"unicode"
)

// minAddressLen Минимально осмысленная длина адреса.
const minAddressLen = 10

// Address Адрес объекта аренды.
type Address string

// NewAddress Проверяет адрес и создаёт value object.
func NewAddress(raw string) (Address, error) {
	address := Address(strings.Join(strings.Fields(raw), " "))

	if len([]rune(address)) < minAddressLen {
		return "", fmt.Errorf("%w: адрес слишком короткий, укажите город, улицу и дом", ErrInvalidAnswer)
	}

	// Проверяем, что в адресе есть хотя бы что-то кроме пунктуации.
	var hasLetters bool
	for _, r := range address {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasLetters = true

			break
		}
	}

	if !hasLetters {
		return "", fmt.Errorf("%w: адрес должен содержать буквы или цифры", ErrInvalidAnswer)
	}

	return address, nil
}

// String Возвращает адрес строкой.
func (a Address) String() string {
	return string(a)
}
