package listing

import (
	"fmt"
	"strings"
)

// Utilities Кто оплачивает коммунальные платежи.
type Utilities string

// Варианты оплаты коммунальных.
const (
	// UtilitiesTenant Платит арендатор.
	UtilitiesTenant Utilities = "tenant"
	// UtilitiesLandlord Платит арендодатель.
	UtilitiesLandlord Utilities = "landlord"
	// UtilitiesShared Платим поровну.
	UtilitiesShared Utilities = "shared"
)

// Payload вариантов оплаты для кнопок.
const (
	// PayloadUtilitiesTenant payload кнопки «арендатор».
	PayloadUtilitiesTenant = "utilities_tenant"
	// PayloadUtilitiesLandlord payload кнопки «арендодатель».
	PayloadUtilitiesLandlord = "utilities_landlord"
	// PayloadUtilitiesShared payload кнопки «поровну».
	PayloadUtilitiesShared = "utilities_shared"
)

// ParseUtilitiesAnswer Разбирает ответ на вопрос о коммунальных:
// payload кнопки или само значение.
func ParseUtilitiesAnswer(answer string) (Utilities, error) {
	switch Utilities(strings.TrimSpace(answer)) {
	case UtilitiesTenant, PayloadUtilitiesTenant:
		return UtilitiesTenant, nil
	case UtilitiesLandlord, PayloadUtilitiesLandlord:
		return UtilitiesLandlord, nil
	case UtilitiesShared, PayloadUtilitiesShared:
		return UtilitiesShared, nil
	default:
		return "", fmt.Errorf("%w: выберите вариант на кнопке", ErrInvalidAnswer)
	}
}

// Title Возвращает вариант для показа пользователю.
func (u Utilities) Title() string {
	switch u {
	case UtilitiesTenant:
		return "оплачивает арендатор"
	case UtilitiesLandlord:
		return "оплачивает арендодатель"
	case UtilitiesShared:
		return "оплачиваем поровну"
	default:
		return string(u)
	}
}
