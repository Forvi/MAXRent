// Package listing Домен заявки арендодателя на сдачу жилья.
package listing

import (
	"errors"
	"fmt"
	"strings"
	"time"

	domainuser "github.com/Forvi/maxrent/internal/domain/user"
)

// Ошибки домена заявки.
var (
	// ErrNotFound Заявка не найдена.
	ErrNotFound = errors.New("listing not found")
	// ErrCodeTaken Код заявки уже занят.
	ErrCodeTaken = errors.New("listing code is taken")
	// ErrInvalidAnswer Ответ не проходит валидацию.
	ErrInvalidAnswer = errors.New("invalid answer")
	// ErrAlreadyPaired Заявка уже занята арендатором.
	ErrAlreadyPaired = errors.New("listing already has a tenant")
	// ErrNoActiveListing У арендодателя нет активной заявки.
	ErrNoActiveListing = errors.New("no active listing")
	// ErrSameUser Арендатор не может присоединиться к собственной заявке.
	ErrSameUser = errors.New("landlord cannot be the tenant")
)

// ID Идентификатор заявки.
type ID int64

// String Возвращает строковое представление идентификатора.
func (id ID) String() string {
	return fmt.Sprintf("%d", int64(id))
}

// Listing Заявка арендодателя: объект и условия аренды до подключения арендатора.
type Listing struct {
	// ID идентификатор заявки.
	ID ID
	// Code код для поиска заявки арендатором, заполняется при публикации.
	Code Code
	// LandlordID арендодатель, создавший заявку.
	LandlordID domainuser.ID
	// TenantID арендатор, подключившийся к заявке; nil пока не подключился.
	TenantID *domainuser.ID
	// Status состояние заявки.
	Status Status
	// Step текущий вопрос анкеты.
	Step Step
	// Address адрес объекта.
	Address Address
	// Price стоимость аренды в месяц.
	Price Money
	// Deposit залог.
	Deposit Money
	// Term срок найма.
	Term Term
	// Utilities кто оплачивает коммунальные.
	Utilities Utilities
	// Description свободное описание, необязательное.
	Description string
	// Сведения сторон для договора. Пустые, пока анкета не заполнена.
	ContractData ContractData
	// CreatedAt момент создания.
	CreatedAt time.Time
	// UpdatedAt момент последнего изменения.
	UpdatedAt time.Time
}

// Draft Создаёт новую черновую заявку арендодателя.
func Draft(landlordID domainuser.ID, now time.Time) Listing {
	return Listing{
		LandlordID: landlordID,
		Status:     StatusDraft,
		Step:       StepAddress,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// IsActive Сообщает, заявка ещё не закрыта.
func (l Listing) IsActive() bool {
	return l.Status == StatusDraft || l.Status == StatusPublished
}

// IsDraft Сообщает, идёт ли заполнение анкеты.
func (l Listing) IsDraft() bool {
	return l.Status == StatusDraft
}

// HasCode Сообщает, выдан ли код: у черновика его ещё нет.
func (l Listing) HasCode() bool {
	return l.Code != ""
}

// Complete заполняет анкету и публикует заявку с указанным кодом.
func (l Listing) Complete(code Code, now time.Time) Listing {
	l.Code = code
	l.Status = StatusPublished
	l.Step = StepDone
	l.UpdatedAt = now

	return l
}

// WithTenant Подключает арендатора к заявке.
// WithContractData Возвращает копию заявки с указанными сведениями сторон.
func (l Listing) WithContractData(data ContractData) Listing {
	l.ContractData = data

	return l
}

// WithTenant Возвращает копию заявки, подключённой к арендатору.
func (l Listing) WithTenant(tenantID domainuser.ID, now time.Time) Listing {
	l.TenantID = &tenantID
	l.Status = StatusPaired
	l.UpdatedAt = now

	return l
}

// Cancelled Возвращает отменённую заявку.
func (l Listing) Cancelled(now time.Time) Listing {
	l.Status = StatusCancelled
	l.UpdatedAt = now

	return l
}

// ApplyAnswer Применяет ответ пользователя к текущему шагу анкеты.
// Валидация живёт в домене: сервис только сохраняет результат.
func (l Listing) ApplyAnswer(answer string) (Listing, error) {
	trimmed := strings.TrimSpace(answer)

	// Ответ относится к текущему шагу, а следующий вычисляется после.
	switch l.Step {
	case StepAddress:
		address, err := NewAddress(trimmed)
		if err != nil {
			return l, err
		}

		l.Address = address
	case StepPrice:
		price, err := ParseMoney(trimmed)
		if err != nil {
			return l, err
		}

		// Нулевая аренда — ошибка ввода, а не «бесплатно».
		if !price.Positive() {
			return l, fmt.Errorf("%w: аренда не может быть нулевой", ErrInvalidAnswer)
		}

		l.Price = price
	case StepDeposit:
		deposit, err := ParseMoney(trimmed)
		if err != nil {
			return l, err
		}

		l.Deposit = deposit
	case StepTerm:
		term, err := ParseTermAnswer(trimmed)
		if err != nil {
			return l, err
		}

		l.Term = term
	case StepUtilities:
		utilities, err := ParseUtilitiesAnswer(trimmed)
		if err != nil {
			return l, err
		}

		l.Utilities = utilities
	case StepDescription:
		if len([]rune(trimmed)) > maxDescriptionLen {
			return l, fmt.Errorf("%w: описание длиннее %d символов", ErrInvalidAnswer, maxDescriptionLen)
		}

		l.Description = trimmed
	case StepDone:
		return l, fmt.Errorf("%w: анкета уже заполнена", ErrInvalidAnswer)
	}

	next, err := l.Step.Next()
	if err != nil {
		return l, err
	}

	l.Step = next

	return l, nil
}

// maxDescriptionLen Ограничение длины свободного описания.
const maxDescriptionLen = 1000
