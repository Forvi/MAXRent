package contract_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
)

func TestNewPerson(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		phone    string
		wantErr  bool
	}{
		{name: "обычный", fullName: "Иванов Иван Иванович", phone: "+7 926 123-45-67"},
		{name: "лишние пробелы", fullName: "  Петрова   Анна  ", phone: "89031234567"},
		{name: "короткое имя", fullName: "Иван", wantErr: true},
		{name: "пустое имя", fullName: "", wantErr: true},
		{name: "телефон из букв", fullName: "Иванов Иван", phone: "звоните", wantErr: true},
		{name: "короткий телефон", fullName: "Иванов Иван", phone: "123", wantErr: true},
		{name: "пустой телефон", fullName: "Иванов Иван", phone: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := contract.NewPerson(tt.fullName, tt.phone)

			if tt.wantErr {
				if !errors.Is(err, contract.ErrInvalidAnswer) {
					t.Fatalf("ожидалась ErrInvalidAnswer, получено %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}

			if got.FullName == "" {
				t.Error("ФИО не должно быть пустым")
			}
		})
	}
}

func TestPartyTitles(t *testing.T) {
	if got := contract.PartyLandlord.Title(); got != "Наймодатель" {
		t.Errorf("название наймодателя: %q", got)
	}

	if got := contract.PartyTenant.Title(); got != "Наниматель" {
		t.Errorf("название нанимателя: %q", got)
	}

	if contract.PartyTenant.Other() != contract.PartyLandlord {
		t.Error("противоположная сторона для нанимателя — наймодатель")
	}
}

func TestDocumentReadiness(t *testing.T) {
	l := listing.Draft(domainuser.NewID(1), time.Now())
	tenant, _ := contract.NewPerson("Иванов Иван Иванович", "89031234567")
	landlord, _ := contract.NewPerson("Петрова Анна Сергеевна", "89037654321")

	// Без кода заявка ещё не опубликована — договор не сформировать.
	l.Code = ""
	doc := contract.NewDocument(l, tenant, landlord, time.Now())
	if doc.IsReady() {
		t.Error("договор без кода заявки не должен считаться готовым")
	}

	// Без данных одной из сторон.
	l.Code = "123456"
	doc = contract.NewDocument(l, contract.Person{}, landlord, time.Now())
	if doc.IsReady() {
		t.Error("договор без данных нанимателя не должен считаться готовым")
	}

	missing := doc.MissingFields()
	if len(missing) != 1 || missing[0] != "Наниматель" {
		t.Errorf("не указано, чьих данных не хватает: %v", missing)
	}

	// Полные данные.
	doc = contract.NewDocument(l, tenant, landlord, time.Now())
	if !doc.IsReady() {
		t.Error("договор с полными данными должен быть готов")
	}
	if len(doc.MissingFields()) != 0 {
		t.Errorf("нечего не должно не хватать: %v", doc.MissingFields())
	}
}

func TestTermMonths(t *testing.T) {
	l := listing.Draft(domainuser.NewID(1), time.Now())
	tenant, _ := contract.NewPerson("Иванов Иван", "89031234567")
	landlord, _ := contract.NewPerson("Петрова Анна", "89037654321")

	l.Term = listing.TermShort
	short := contract.NewDocument(l, tenant, landlord, time.Now())
	if short.TermMonths() >= 12 {
		t.Errorf("краткосрочный найм должен быть меньше года, получено %d", short.TermMonths())
	}

	l.Term = listing.TermLong
	long := contract.NewDocument(l, tenant, landlord, time.Now())
	if long.TermMonths() != 12 {
		t.Errorf("долгосрочный найм ожидался 12 месяцев, получено %d", long.TermMonths())
	}
}
