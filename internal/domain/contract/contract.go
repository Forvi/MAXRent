// Package contract Домен договора найма жилого помещения.
package contract

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Forvi/maxrent/internal/domain/listing"
	"github.com/Forvi/maxrent/internal/domain/party"
)

// Ошибки домена договора.
var (
	// ErrNotReady Данных для формирования договора недостаточно.
	ErrNotReady = errors.New("contract is not ready")
	// ErrInvalidAnswer Ответ не проходит валидацию.
	ErrInvalidAnswer = errors.New("invalid answer")
)

// Party Сторона договора. Определена в пакете party, потому что нужна
// и заявке, и договору.
type Party = party.Party

// Стороны договора.
const (
	// PartyTenant Наниматель — арендатор жилья.
	PartyTenant = party.Tenant
	// PartyLandlord Нарендодатель — собственник, сдающий жильё.
	PartyLandlord = party.Landlord
)

// Person Сведения о человеке, заполняемые анкетой.
type Person struct {
	// FullName фамилия, имя, отчество.
	FullName string
	// Phone контактный телефон.
	Phone string
}

// Минимальные требования к вводимым данным.
const (
	minNameRunes   = 3
	minPhoneDigits = 5
)

// NewPerson Создаёт и проверяет сведения о человеке.
func NewPerson(fullName, phone string) (Person, error) {
	name, err := ValidateName(fullName)
	if err != nil {
		return Person{}, err
	}

	number, err := ValidatePhone(phone)
	if err != nil {
		return Person{}, err
	}

	return Person{FullName: name, Phone: number}, nil
}

// ValidateName Нормализует и проверяет ФИО.
// Поля анкеты собираются по одному, поэтому проверяются по отдельно.
//
// Требуется хотя бы одна буква: иначе шестизначный код заявки, введённый
// арендатором, прошёл бы проверку длины и сохранился бы как ФИО.
func ValidateName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if len([]rune(name)) < minNameRunes {
		return "", fmt.Errorf("%w: укажите фамилию, имя и отчество", ErrInvalidAnswer)
	}

	if !hasLetter(name) {
		return "", fmt.Errorf("%w: ФИО должно содержать буквы", ErrInvalidAnswer)
	}

	return name, nil
}

// hasLetter Сообщает, есть ли в строке хотя бы одна буква.
func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}

	return false
}

// ValidatePhone Проверяет телефон: считает цифры, чтобы «звоните» не прошло.
func ValidatePhone(raw string) (string, error) {
	digits := 0
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits++
		}
	}

	if digits < minPhoneDigits {
		return "", fmt.Errorf("%w: телефон должен содержать не меньше %d цифр",
			ErrInvalidAnswer, minPhoneDigits)
	}

	return strings.TrimSpace(raw), nil
}

// IsEmpty Сообщает, что человек ещё не заполнил анкету.
func (p Person) IsEmpty() bool {
	return p.FullName == "" && p.Phone == ""
}

// Document Договор найма, собранный из данных обеих сторон.
type Document struct {
	// Listing заявка, из которой взяты условия аренды.
	Listing listing.Listing
	// Tenant сведения о нанимателе.
	Tenant Person
	// Landlord сведения о наймодателе.
	Landlord Person
	// CreatedAt дата формирования документа.
	CreatedAt time.Time
}

// NewDocument Собирает договор из заявки и сведений сторон.
func NewDocument(l listing.Listing, tenant, landlord Person, createdAt time.Time) Document {
	return Document{
		Listing:   l,
		Tenant:    tenant,
		Landlord:  landlord,
		CreatedAt: createdAt,
	}
}

// IsReady Сообщает, собраны ли все данные.
func (d Document) IsReady() bool {
	return !d.Tenant.IsEmpty() && !d.Landlord.IsEmpty() && d.Listing.HasCode()
}

// MissingFields Перечисляет стороны, по которым данных не хватает.
func (d Document) MissingFields() []string {
	var missing []string

	if d.Tenant.IsEmpty() {
		missing = append(missing, PartyTenant.Title())
	}

	if d.Landlord.IsEmpty() {
		missing = append(missing, PartyLandlord.Title())
	}

	return missing
}

// FileName Имя файла договора.
func (d Document) FileName() string {
	return fmt.Sprintf("Договор_найма_%s.pdf", d.Listing.Code)
}

// TermMonths Срок найма в месяцах, выведенный из заявки.
// В документе это ориентир: точные даты стороны проставляют от руки.
func (d Document) TermMonths() int {
	if d.Listing.Term == listing.TermLong {
		return 12
	}

	return 11
}
