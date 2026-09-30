package listing

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Money Денежная сумма в копейках. Целые рубли в хранении приводят к
// ошибкам округления, а float в деньгах недопустим.
type Money int64

// ParseMoney Разбирает сумму, введённую пользователем.
// Принимает «35000», «35 000», «35000,50» и неразрывные пробелы.
func ParseMoney(raw string) (Money, error) {
	// Убираем все пробельные символы, включая неразрывные: пользователь
	// нередко вставляет «35 000» с неразрывным пробелом с телефона.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}

		return r
	}, raw)
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.ReplaceAll(cleaned, ",", ".")

	if cleaned == "" {
		return 0, fmt.Errorf("%w: введите число", ErrInvalidAnswer)
	}

	// Минус отсекаем явно: отрицательные суммы в заявке недопустимы.
	if strings.HasPrefix(cleaned, "-") {
		return 0, fmt.Errorf("%w: сумма не может быть отрицательной", ErrInvalidAnswer)
	}

	whole, frac, hasFrac := strings.Cut(cleaned, ".")
	if whole == "" {
		return 0, fmt.Errorf("%w: введите число", ErrInvalidAnswer)
	}

	rubles, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: введите число", ErrInvalidAnswer)
	}

	var kopecks int64
	if hasFrac {
		switch len(frac) {
		case 1:
			digit, convErr := strconv.ParseInt(frac, 10, 64)
			if convErr != nil {
				return 0, fmt.Errorf("%w: неверная дробная часть", ErrInvalidAnswer)
			}

			kopecks = digit * 10
		case 2:
			digits, convErr := strconv.ParseInt(frac, 10, 64)
			if convErr != nil {
				return 0, fmt.Errorf("%w: неверная дробная часть", ErrInvalidAnswer)
			}

			kopecks = digits
		default:
			return 0, fmt.Errorf("%w: не больше двух знаков после запятой", ErrInvalidAnswer)
		}
	}

	return Money(rubles*100 + kopecks), nil
}

// String Возвращает сумму в виде числа: «35000» или «35000,50».
func (m Money) String() string {
	rubles := int64(m) / 100
	kopecks := int64(m) % 100

	if kopecks == 0 {
		return strconv.FormatInt(rubles, 10)
	}

	return fmt.Sprintf("%d,%02d", rubles, kopecks)
}

// Positive Сообщает, что сумма больше нуля.
func (m Money) Positive() bool {
	return m > 0
}

// NonNegative Сообщает, что сумма не отрицательна.
func (m Money) NonNegative() bool {
	return m >= 0
}
