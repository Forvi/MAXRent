package listing

import (
	"fmt"
	"strings"
)

// Term Срок найма.
type Term string

// Сроки найма: до года — краткосрочный, от года — долгосрочный.
const (
	// TermShort Наём до года.
	TermShort Term = "short"
	// TermLong Наём от года.
	TermLong Term = "long"
)

// Payload сроков для кнопок.
const (
	// PayloadTermShort payload кнопки «до года».
	PayloadTermShort = "term_short"
	// PayloadTermLong payload кнопки «от года».
	PayloadTermLong = "term_long"
)

// ParseTermAnswer Разбирает ответ на вопрос о сроке: payload кнопки или само значение.
func ParseTermAnswer(answer string) (Term, error) {
	switch Term(strings.TrimSpace(answer)) {
	case TermShort, PayloadTermShort:
		return TermShort, nil
	case TermLong, PayloadTermLong:
		return TermLong, nil
	default:
		return "", fmt.Errorf("%w: выберите срок на кнопке", ErrInvalidAnswer)
	}
}

// Title Возвращает срок для показа пользователю.
func (t Term) Title() string {
	switch t {
	case TermShort:
		return "до года"
	case TermLong:
		return "от года"
	default:
		return string(t)
	}
}
