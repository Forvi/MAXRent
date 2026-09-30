package listing

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// CodeLength Длина кода заявки.
const CodeLength = 6

// codeAlphabet Границы диапазона: шестизначные числа без ведущих нулей,
// чтобы «012345» не путать с «12345».
const (
	codeMin = 100000
	codeMax = 999999
)

// Code Код заявки, по которому её находит арендатор.
type Code string

// GenerateCode Создаёт случайный шестизначный код.
// Используется crypto/rand: math/rand не годится для кода, по которому
// открывается чужая сделка.
func GenerateCode() (Code, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeMax-codeMin+1))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}

	return Code(fmt.Sprintf("%06d", n.Int64()+codeMin)), nil
}

// ParseCode Разбирает код, введённый пользователем.
func ParseCode(raw string) (Code, error) {
	code := Code(strings.TrimSpace(raw))
	if len(code) != CodeLength {
		return "", fmt.Errorf("%w: код должен содержать %d цифр", ErrInvalidAnswer, CodeLength)
	}

	for _, r := range code {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("%w: код состоит только из цифр", ErrInvalidAnswer)
		}
	}

	return code, nil
}

// String Возвращает код строкой.
func (c Code) String() string {
	return string(c)
}
