package listing

import "github.com/Forvi/maxrent/internal/domain/party"

// ContractData Сведения сторон, нужные для формирования договора.
// Хранятся рядом с заявкой: договор собирается из условий заявки и этих данных.
type ContractData struct {
	// TenantFullName ФИО нанимателя.
	TenantFullName string
	// TenantPhone телефон нанимателя.
	TenantPhone string
	// LandlordFullName ФИО наймодателя.
	LandlordFullName string
	// LandlordPhone телефон наймодателя.
	LandlordPhone string
	// DocumentToken токен загруженного в мессенджер файла договора.
	DocumentToken string
}

// HasTenant Сообщает, что данные нанимателя собраны полностью.
func (c ContractData) HasTenant() bool {
	return c.TenantFullName != "" && c.TenantPhone != ""
}

// HasLandlord Сообщает, что данные наймодателя собраны полностью.
func (c ContractData) HasLandlord() bool {
	return c.LandlordFullName != "" && c.LandlordPhone != ""
}

// IsComplete Сообщает, что данные обеих сторон собраны.
func (c ContractData) IsComplete() bool {
	return c.HasTenant() && c.HasLandlord()
}

// Name Возвращает ФИО указанной стороны.
func (c ContractData) Name(who party.Party) string {
	if who == party.Landlord {
		return c.LandlordFullName
	}

	return c.TenantFullName
}

// Phone Возвращает телефон указанной стороны.
func (c ContractData) Phone(who party.Party) string {
	if who == party.Landlord {
		return c.LandlordPhone
	}

	return c.TenantPhone
}

// SetName Записывает ФИО указанной стороны.
func (c *ContractData) SetName(who party.Party, name string) {
	if who == party.Landlord {
		c.LandlordFullName = name

		return
	}

	c.TenantFullName = name
}

// SetPhone Записывает телефон указанной стороны.
func (c *ContractData) SetPhone(who party.Party, phone string) {
	if who == party.Landlord {
		c.LandlordPhone = phone

		return
	}

	c.TenantPhone = phone
}
