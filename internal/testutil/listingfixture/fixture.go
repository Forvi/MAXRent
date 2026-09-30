// Package listingfixture Готовые сценарии заявок для тестов.
// Вынесен отдельно, потому что один и тот же черновик нужен тестам домена,
// сервиса и обработчика, а тестовые пакеты Go привязаны к своему каталогу.
package listingfixture

import (
	"testing"
	"time"

	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
)

// Идентификаторы пользователей по умолчанию.
var (
	// LandlordID Арендодатель, создающий заявку.
	LandlordID = domainuser.NewID(101)
	// TenantID Арендатор, подключающийся по коду.
	TenantID = domainuser.NewID(202)
	// Code Код опубликованной заявки.
	Code = listing.Code("123456")
)

// Answers Ответы, проводящие черновик до публикации.
func Answers() map[listing.Step]string {
	return map[listing.Step]string{
		listing.StepAddress:     "Москва, ул. Тверская, д. 1, кв. 5",
		listing.StepPrice:       "35000",
		listing.StepDeposit:     "35000",
		listing.StepTerm:        listing.PayloadTermShort,
		listing.StepUtilities:   listing.PayloadUtilitiesShared,
		listing.StepDescription: "Свежий ремонт, мебель остаётся",
	}
}

// Draft Новый черновик на первом шаге.
func Draft() listing.Listing {
	draft := listing.Draft(LandlordID, time.Now())
	draft.ID = 1

	return draft
}

// DraftAtStep Черновик, заполненный до указанного шага.
func DraftAtStep(t *testing.T, step listing.Step) listing.Listing {
	t.Helper()

	return fill(t, Draft(), step)
}

// Published Опубликованная заявка с выданным кодом.
func Published(t *testing.T) listing.Listing {
	t.Helper()

	return fill(t, Draft(), listing.StepDone).Complete(Code, time.Now())
}

// Paired Опубликованная заявка, к которой уже подключился арендатор.
func Paired(t *testing.T) listing.Listing {
	t.Helper()

	return Published(t).WithTenant(TenantID, time.Now())
}

// fill Заполняет черновик ответами до указанного шага.
func fill(t *testing.T, draft listing.Listing, step listing.Step) listing.Listing {
	t.Helper()

	answers := Answers()
	for draft.Step != step && draft.Step != listing.StepDone {
		answer, ok := answers[draft.Step]
		if !ok {
			t.Fatalf("нет ответа для шага %s", draft.Step)
		}

		updated, err := draft.ApplyAnswer(answer)
		if err != nil {
			t.Fatalf("шаг %s: %v", draft.Step, err)
		}

		draft = updated
	}

	return draft
}
