package app_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/Forvi/maxrent/internal/document/pdf"
	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	contracthandler "github.com/Forvi/maxrent/internal/handlers/contract"
	listinghandler "github.com/Forvi/maxrent/internal/handlers/listing"
	"github.com/Forvi/maxrent/internal/infrastructure/bot"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports/mocks"
	contractservice "github.com/Forvi/maxrent/internal/services/contract"
	listingservice "github.com/Forvi/maxrent/internal/services/listing"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

const (
	landlordChat = int64(10)
	landlordUID  = int64(101)
	tenantChat   = int64(20)
	tenantUID    = int64(202)

	listingCode = listing.Code("123456")
)

// env Собранная цепочка обработчиков приложения и её заглушки.
type env struct {
	router   *bot.Router
	listings *mocks.MockListingRepository
	parties  *mocks.MockContractPartyRepository
	users    *mocks.MockUserRepository
	sender   *mocks.MockMessageSender
	ctx      context.Context
}

// newEnv Собирает обработчики в том же порядке, что и composition root.
func newEnv(t *testing.T) *env {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	listings := mocks.NewMockListingRepository(t)
	users := mocks.NewMockUserRepository(t)
	sender := mocks.NewMockMessageSender(t)

	parties := &mocks.MockContractPartyRepository{}
	parties.Test(t)
	t.Cleanup(func() { parties.AssertExpectations(t) })

	userSvc := userservice.NewService(users, log)

	// Порядок как в internal/app/dependency.go: заявка раньше договора.
	handlers := []bot.Handler{
		listinghandler.NewListingHandler(
			listingservice.NewService(listings, log), userSvc, sender, log,
		),
		contracthandler.NewHandler(
			contractservice.NewService(parties, pdf.NewGenerator(), log),
			listings, userSvc, sender, log,
		),
	}

	return &env{
		router:   bot.NewRouter(log, handlers...),
		listings: listings,
		parties:  parties,
		users:    users,
		sender:   sender,
		ctx:      context.Background(),
	}
}

// send Прогоняет событие через цепочку обработчиков.
func (e *env) send(update maxapi.Update) {
	e.router.Route(e.ctx, update)
}

// expectUserGet Заглушка чтения пользователя.
//
// times: один обработчик читает роль пользователя, но когда первый отказывается
// от сообщения, роль читает следующий. Поэтому число чтений равно числу
// обработчиков, дошедших до своей логики.
func (e *env) expectUserGet(u domainuser.User, times int) {
	e.users.EXPECT().FindByID(mock.Anything, u.ID).Return(u, nil).Times(times)
}

// text Событие с обычным текстом: команда заполняется так же, как это делает MAX.
func text(chatID, userID int64, body string) maxapi.Update {
	update := maxapi.Update{
		Type:   maxapi.UpdateMessageCreated,
		ChatID: chatID,
		UserID: userID,
		Text:   body,
	}

	if strings.HasPrefix(body, "/") {
		update.Command = maxapi.Command{Name: body}
	}

	return update
}

func landlord() domainuser.User {
	return domainuser.New(domainuser.NewID(landlordUID), time.Now()).WithRole(domainuser.RoleLandlord)
}

func tenant() domainuser.User {
	return domainuser.New(domainuser.NewID(tenantUID), time.Now()).WithRole(domainuser.RoleTenant)
}

// landlordDraft Заявка арендодателя, у которой спрашивают цену.
func landlordDraft(t *testing.T) listing.Listing {
	t.Helper()

	l := listing.Draft(domainuser.NewID(landlordUID), time.Now())
	l.ID = 1
	l.Code = listingCode
	l.Address = "Москва, ул. Тверская, д. 1, кв. 5"
	l.Price = 3500000
	l.Deposit = 3500000
	l.Term = listing.TermLong
	l.Utilities = listing.UtilitiesTenant
	l.Step = listing.StepPrice

	return l
}

// publishedListing Опубликованная заявка без сведений сторон.
func publishedListing(t *testing.T, data listing.ContractData) listing.Listing {
	t.Helper()

	l := listing.Draft(domainuser.NewID(landlordUID), time.Now())
	l.ID = 1
	l.Code = listingCode
	l.Address = "Москва, ул. Тверская, д. 1, кв. 5"
	l.Price = 3500000
	l.Deposit = 3500000
	l.Term = listing.TermLong
	l.Utilities = listing.UtilitiesTenant
	l.Step = listing.StepDone
	l.Status = listing.StatusPublished
	l.ContractData = data

	return l
}

func pairedForTenant(t *testing.T, data listing.ContractData) listing.Listing {
	t.Helper()

	l := publishedListing(t, data)
	id := domainuser.NewID(tenantUID)
	l.TenantID = &id
	l.Status = listing.StatusPaired

	return l
}

func TestLandlordAddressDoesNotLeakIntoContractQuestionnaire(t *testing.T) {
	e := newEnv(t)
	draft := landlordDraft(t)

	e.expectUserGet(landlord(), 1)
	e.listings.EXPECT().FindActiveByLandlord(mock.Anything, domainuser.NewID(landlordUID)).
		Return(draft, nil).Once()

	// Анкета заявки спрашивает цену.
	e.sender.EXPECT().SendMessage(e.ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "Сколько стоит")
	})).Return(nil).Once()

	e.send(text(landlordChat, landlordUID, "Москва, Ленина, 8, 48"))

	// Сведений сторон в базе не появилось: адрес ушёл только в заявку.
	// Раньше он сохранялся ещё и как ФИО договора.
	e.parties.AssertNotCalled(t, "SetPartyData", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	e.sender.AssertNumberOfCalls(t, "SendMessage", 1)
}

func TestLandlordPlainTextGoesToContractWhenListingPublished(t *testing.T) {
	e := newEnv(t)
	published := publishedListing(t, listing.ContractData{})

	e.expectUserGet(landlord(), 2)
	e.listings.EXPECT().FindActiveByLandlord(mock.Anything, domainuser.NewID(landlordUID)).
		Return(published, nil).Twice()

	// Сведения сторон пусты — этот текст принадлежит анкете договора.
	// Текст сохранён как ФИО наймодателя, бот перешёл к телефону.
	e.parties.EXPECT().SetPartyData(e.ctx, listing.ID(1), mock.Anything, mock.MatchedBy(
		func(d listing.ContractData) bool { return d.LandlordFullName == "Петрова Анна" },
	)).Return(nil).Once()
	e.sender.EXPECT().SendMessage(e.ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "телефон")
	})).Return(nil).Once()

	e.send(text(landlordChat, landlordUID, "Петрова Анна"))
}

func TestTenantCodeIsNotSavedAsContractName(t *testing.T) {
	e := newEnv(t)
	published := publishedListing(t, listing.ContractData{})

	e.expectUserGet(tenant(), 1)
	// Арендатор ещё не подключён — код ищем.
	e.listings.EXPECT().FindActiveByTenant(e.ctx, domainuser.NewID(tenantUID)).
		Return(listing.Listing{}, listing.ErrNoActiveListing).Once()
	e.listings.EXPECT().FindByCode(e.ctx, listingCode).Return(published, nil).Once()
	e.sender.EXPECT().SendMessageWithKeyboard(e.ctx, tenantChat, mock.Anything, mock.Anything).
		Return(nil).Once()

	e.send(text(tenantChat, tenantUID, "123456"))

	// Код не должен стать ФИО: раньше анкета договора принимала его как имя.
	e.parties.AssertNotCalled(t, "SetPartyData", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	e.sender.AssertNumberOfCalls(t, "SendMessageWithKeyboard", 1)
	e.sender.AssertNumberOfCalls(t, "SendMessage", 0)
}

func TestPairedTenantCodeGoesToContractQuestionnaire(t *testing.T) {
	e := newEnv(t)
	paired := pairedForTenant(t, listing.ContractData{})

	e.expectUserGet(tenant(), 2)
	// Уже подключён: чужой код не ищем, текст уходит в анкету договора.
	e.listings.EXPECT().FindActiveByTenant(e.ctx, domainuser.NewID(tenantUID)).
		Return(paired, nil).Once()

	// Текст сохранён как ФИО арендатора, бот перешёл к телефону.
	e.parties.EXPECT().SetPartyData(e.ctx, listing.ID(1), mock.Anything, mock.MatchedBy(
		func(d listing.ContractData) bool { return d.TenantFullName == "Сидоров Пётр" },
	)).Return(nil).Once()
	e.sender.EXPECT().SendMessage(e.ctx, tenantChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "телефон")
	})).Return(nil).Once()

	e.send(text(tenantChat, tenantUID, "Сидоров Пётр"))
	e.sender.AssertNumberOfCalls(t, "SendMessage", 1)
}

func TestOneMessageOneQuestion(t *testing.T) {
	e := newEnv(t)
	draft := landlordDraft(t)

	e.expectUserGet(landlord(), 1)
	e.listings.EXPECT().FindActiveByLandlord(mock.Anything, domainuser.NewID(landlordUID)).
		Return(draft, nil).Once()
	e.sender.EXPECT().SendMessage(e.ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "Сколько стоит")
	})).Return(nil).Once()

	e.send(text(landlordChat, landlordUID, "Москва, Ленина, 8, 48"))

	// Ровно один вопрос на одно сообщение — исходная жалоба.
	e.sender.AssertNumberOfCalls(t, "SendMessage", 1)
}

func TestCompletedQuestionnaireLeavesPlainTextAlone(t *testing.T) {
	e := newEnv(t)
	filled := publishedListing(t, listing.ContractData{
		TenantFullName:   "Иванов Иван Иванович",
		TenantPhone:      "89031234567",
		LandlordFullName: "Петрова Анна",
		LandlordPhone:    "89037654321",
	})

	e.expectUserGet(landlord(), 2)
	e.listings.EXPECT().FindActiveByLandlord(mock.Anything, domainuser.NewID(landlordUID)).
		Return(filled, nil).Twice()

	// Анкета заполнена и заявка опубликована: сообщение не наше ни одному
	// обработчику, поэтому бот молчит вместо повторных подсказок.
	e.send(text(landlordChat, landlordUID, "спасибо"))
	e.sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
}
