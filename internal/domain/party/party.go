// Package party Стороны сделки аренды жилья.
// Тип вынесен отдельно, потому что используется и в заявке, и в договоре:
// иначе возник бы циклический импорт между доменами.
package party

// Party Сторона сделки.
type Party string

// Стороны сделки.
const (
	// Tenant Арендатор — наниматель жилья.
	Tenant Party = "tenant"
	// Landlord Арендодатель — собственник, сдающий жильё.
	Landlord Party = "landlord"
)

// Title Возвращает роль в том виде, как она названа в тексте договора.
func (p Party) Title() string {
	if p == Landlord {
		return "Наймодатель"
	}

	return "Наниматель"
}

// TitleGenitive Возвращает роль в родительном падеже.
// Нужна в сообщениях вида «жду данные наймодателя» — иначе предложение
// получается грамматически неверным.
func (p Party) TitleGenitive() string {
	if p == Landlord {
		return "наймодателя"
	}

	return "нанимателя"
}

// Other Возвращает противоположную сторону.
func (p Party) Other() Party {
	if p == Landlord {
		return Tenant
	}

	return Landlord
}
