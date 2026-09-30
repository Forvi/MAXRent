package pdf_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Forvi/maxrent/internal/document/pdf"
	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
)

// readyDocument Собирает договор с полными данными обеих сторон.
func readyDocument(t *testing.T) contract.Document {
	t.Helper()

	l := listing.Draft(domainuser.NewID(1), time.Now())
	l.Code = "123456"
	l.Address = "Москва, ул. Тверская, д. 1, кв. 5"
	l.Price = 3500000
	l.Deposit = 3500050
	l.Term = listing.TermLong
	l.Utilities = listing.UtilitiesTenant
	l.Description = "Свежий ремонт"

	tenant, err := contract.NewPerson("Иванов Иван Иванович", "+7 926 123-45-67")
	requireNoError(t, err)

	landlord, err := contract.NewPerson("Петрова Анна Сергеевна", "+7 903 765-43-21")
	requireNoError(t, err)

	return contract.NewDocument(l, tenant, landlord, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))
}

func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderProducesPDF(t *testing.T) {
	out, err := pdf.NewGenerator().Render(readyDocument(t))
	requireNoError(t, err)

	if len(out) < 1000 {
		t.Fatalf("PDF подозрительно мал: %d байт", len(out))
	}

	// Сигнатура PDF: без неё файл не откроется в мессенджере.
	if got := string(out[:5]); got != "%PDF-" {
		t.Fatalf("нет сигнатуры PDF, получено %q", got)
	}
}

func TestRenderRefusesIncompleteDocument(t *testing.T) {
	doc := readyDocument(t)
	doc.Tenant = contract.Person{}

	_, err := pdf.NewGenerator().Render(doc)
	if err == nil {
		t.Fatal("ожидалась ошибка при неполных данных")
	}

	if !strings.Contains(err.Error(), "Наниматель") {
		t.Errorf("в ошибке должно быть указано, чьих данных не хватает: %v", err)
	}
}
