package contract_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	contracthandler "github.com/Forvi/maxrent/internal/handlers/contract"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports/mocks"
	contractservice "github.com/Forvi/maxrent/internal/services/contract"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

const (
	landlordChat = int64(10)
	landlordUID  = int64(101)
	tenantUID    = int64(202)
	tenantChat   = int64(20)
)

// fakeGenerator Подставной генератор PDF.
type fakeGenerator struct{ err error }

func (f *fakeGenerator) Render(contract.Document) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}

	return []byte("%PDF-1.7 contract"), nil
}

type fixture struct {
	handler   *contracthandler.Handler
	listings  *mocks.MockListingRepository
	parties   *mocks.MockContractPartyRepository
	sender    *mocks.MockMessageSender
	users     *mocks.MockUserRepository
	generator *fakeGenerator
}

func setup(t *testing.T) *fixture {
	t.Helper()

	listings := mocks.NewMockListingRepository(t)
	sender := mocks.NewMockMessageSender(t)
	users := mocks.NewMockUserRepository(t)
	generator := &fakeGenerator{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	parties := &mocks.MockContractPartyRepository{}
	parties.Test(t)
	t.Cleanup(func() { parties.AssertExpectations(t) })

	contractSvc := contractservice.NewService(parties, generator, log)
	userSvc := userservice.NewService(users, log)

	return &fixture{
		handler:   contracthandler.NewHandler(contractSvc, listings, userSvc, sender, log),
		listings:  listings,
		parties:   parties,
		sender:    sender,
		users:     users,
		generator: generator,
	}
}

func landlord() domainuser.User {
	return domainuser.New(domainuser.NewID(landlordUID), time.Now()).WithRole(domainuser.RoleLandlord)
}

func tenant() domainuser.User {
	return domainuser.New(domainuser.NewID(tenantUID), time.Now()).WithRole(domainuser.RoleTenant)
}

// stubListing Заявка с указанными данными сторон.
func stubListing(data listing.ContractData) listing.Listing {
	l := listing.Draft(domainuser.NewID(landlordUID), time.Now())
	l.ID = 1
	l.Code = "123456"
	l.Address = "Москва, ул. Тверская, д. 1, кв. 5"
	l.Price = 3500000
	l.Deposit = 3500000
	l.Term = listing.TermLong
	l.Utilities = listing.UtilitiesTenant
	// Заявка опубликована: на черновике анкета договора недоступна.
	l.Status = listing.StatusPublished
	l.Step = listing.StepDone
	l.ContractData = data

	return l
}

// pairedStub Заявка, к которой подключён арендатор.
func pairedStub(data listing.ContractData) listing.Listing {
	l := stubListing(data)
	id := domainuser.NewID(tenantUID)
	l.TenantID = &id
	l.Status = listing.StatusPaired

	return l
}

func fullData() listing.ContractData {
	return listing.ContractData{
		TenantFullName:   "Иванов Иван Иванович",
		TenantPhone:      "89031234567",
		LandlordFullName: "Петрова Анна Сергеевна",
		LandlordPhone:    "89037654321",
	}
}

func message(chatID, userID int64, text string) maxapi.Update {
	return maxapi.Update{
		Type:    maxapi.UpdateMessageCreated,
		ChatID:  chatID,
		UserID:  userID,
		Text:    text,
		Command: withCommand(text),
	}
}

// withCommand Заполняет команду так же, как это делает MAX: текст, начинающийся
// со слеша, приходит как команда. Обработчики смотрят именно в Command.Name,
// поэтому тестовые события должны повторять это правило.
func withCommand(text string) maxapi.Command {
	if !strings.HasPrefix(text, "/") {
		return maxapi.Command{}
	}

	return maxapi.Command{Name: text}
}

// expectUserGet Заглушка чтения пользователя.
func (f *fixture) expectUserGet(u domainuser.User) {
	f.users.EXPECT().FindByID(mock.Anything, u.ID).Return(u, nil).Once()
}

func TestDataCommandAsksName(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(listing.ContractData{}), nil).Once()
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(strings.ToLower(s), "фамили")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/data"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestNameThenPhoneThenDone(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(listing.ContractData{}), nil).Once()
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(strings.ToLower(s), "фамили")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/data"))
	require.NoError(t, err)
	require.True(t, handled)

	// Ввод ФИО → просим телефон.
	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(listing.ContractData{}), nil).Once()
	// Матчим конкретно: общий mock.Anything перехватил бы и вызов с телефоном.
	f.parties.EXPECT().SetPartyData(ctx, listing.ID(1), contract.PartyLandlord,
		mock.MatchedBy(func(d listing.ContractData) bool {
			return d.LandlordFullName == "Петрова Анна Сергеевна" && d.LandlordPhone == ""
		})).Return(nil).Once()
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(strings.ToLower(s), "телефон")
	})).Return(nil).Once()

	handled, err = f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "Петрова Анна Сергеевна"))
	require.NoError(t, err)
	require.True(t, handled)

	// Ввод телефона → анкета наймодателя заполнена.
	withName := listing.ContractData{LandlordFullName: "Петрова Анна Сергеевна"}
	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(withName), nil).Once()
	f.parties.EXPECT().SetPartyData(ctx, listing.ID(1), contract.PartyLandlord,
		mock.MatchedBy(func(d listing.ContractData) bool {
			return d.LandlordPhone == "8 903 765-43-21"
		})).Return(nil).Once()
	// Данные наймодателя собраны, но арендатор ещё не заполнил свои —
	// поэтому сообщаем об успехе, а не предлагаем сформировать документ.
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "данные сохранены")
	})).Return(nil).Once()

	handled, err = f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "8 903 765-43-21"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestInvalidNameRepeatsQuestion(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(listing.ContractData{}), nil).Once()
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(strings.ToLower(s), "фамили")
	})).Return(nil).Once()

	// Односимвольное имя не принимается, вопрос повторяется.
	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "И"))
	require.NoError(t, err)
	require.True(t, handled)
	f.parties.AssertNotCalled(t, "SetPartyData", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestContractCommandSendsDocument(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(fullData()), nil).Once()
	f.sender.EXPECT().SendDocument(ctx, landlordChat, "Договор_найма_123456.pdf", mock.Anything).
		Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/contract"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestContractCommandBlockedUntilBothPartiesReady(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	partial := fullData()
	partial.TenantFullName = ""
	partial.TenantPhone = ""

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(partial), nil).Once()
	// Наймодателю не советуют вводить /data: свои данные у него уже есть.
	// Раньше он получал именно такой совет и застревал в цикле подсказок.
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "жду данные нанимателя")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/contract"))
	require.NoError(t, err)
	require.True(t, handled)
	f.sender.AssertNotCalled(t, "SendDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTenantFindsListingByTenant(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	paired := stubListing(fullData())
	tenantID := domainuser.NewID(tenantUID)
	paired.TenantID = &tenantID
	paired.Status = listing.StatusPaired

	f.expectUserGet(tenant())
	// Арендатор ищет заявку по своей привязке, а не как арендодатель.
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).Return(paired, nil).Once()
	f.sender.EXPECT().SendDocument(ctx, tenantChat, "Договор_найма_123456.pdf", mock.Anything).
		Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "/contract"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestNoListingExplainsWhatToDo(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		user  domainuser.User
		chat  int64
		uid   int64
		match string
	}{
		{name: "landlord", user: landlord(), chat: landlordChat, uid: landlordUID, match: "/list"},
		{name: "tenant", user: tenant(), chat: tenantChat, uid: tenantUID, match: "код"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t)

			f.expectUserGet(tt.user)
			f.listings.EXPECT().
				FindActiveByLandlord(mock.Anything, mock.Anything).
				Return(listing.Listing{}, listing.ErrNoActiveListing).
				Maybe()
			f.listings.EXPECT().
				FindActiveByTenant(mock.Anything, mock.Anything).
				Return(listing.Listing{}, listing.ErrNoActiveListing).
				Maybe()
			f.sender.EXPECT().SendMessage(ctx, tt.chat, mock.MatchedBy(func(s string) bool {
				return strings.Contains(s, tt.match)
			})).Return(nil).Once()

			handled, err := f.handler.HandleUpdate(ctx, message(tt.chat, tt.uid, "/data"))
			require.NoError(t, err)
			require.True(t, handled)
		})
	}
}

func TestForeignCommandIgnored(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	// /info обслуживает другая фича — второй ответ пользователю не нужен.
	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/info"))
	require.NoError(t, err)
	// /info обслуживает другая фича: событие не наше.
	require.False(t, handled)
	f.sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
	f.users.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

func TestRenderFailureReportedToUser(t *testing.T) {
	ctx := context.Background()
	f := setup(t)
	f.generator.err = errors.New("font missing")

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(fullData()), nil).Once()
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(strings.ToLower(s), "позже")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/contract"))
	require.NoError(t, err)
	require.True(t, handled)
	f.sender.AssertNotCalled(t, "SendDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestUnregisteredUserIgnored(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	f.users.EXPECT().FindByID(mock.Anything, mock.Anything).
		Return(domainuser.User{}, domainuser.ErrNotFound).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, 999, "/contract"))
	require.NoError(t, err)
	// Незарегистрированный пользователь: сценарий начнётся после /start.
	require.False(t, handled)
	f.sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestTenantWaitsForLandlordWithoutBeingSentInCircles(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	// Данные арендатора есть, наймодателя нет — исходная жалоба.
	partial := fullData()
	partial.LandlordFullName = ""
	partial.LandlordPhone = ""

	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(pairedStub(partial), nil).Once()
	f.sender.EXPECT().SendMessage(ctx, tenantChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(strings.ToLower(s), "жду данные наймодателя")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "/data"))
	require.NoError(t, err)
	require.True(t, handled)
	// Никакого «введите /data» тому, кто его только что ввёл.
	f.sender.AssertNotCalled(t, "SendDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestTenantContractWaitsForLandlord(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	partial := fullData()
	partial.LandlordFullName = ""
	partial.LandlordPhone = ""

	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(pairedStub(partial), nil).Once()
	f.sender.EXPECT().SendMessage(ctx, tenantChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(strings.ToLower(s), "жду данные наймодателя")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "/contract"))
	require.NoError(t, err)
	require.True(t, handled)
	f.sender.AssertNotCalled(t, "SendDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestContractMissingOwnDataTellsToFillIt(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	// Наймодатель не ввёл ничего, арендатор ввёл — обратная ситуация.
	partial := fullData()
	partial.LandlordFullName = ""
	partial.LandlordPhone = ""

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(stubListing(partial), nil).Once()
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "заполните свои данные")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/contract"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestDataCommandRefusedWhileListingIsDraft(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	draft := stubListing(listing.ContractData{})
	draft.Status = listing.StatusDraft
	draft.Step = listing.StepPrice

	f.expectUserGet(landlord())
	f.listings.EXPECT().FindActiveByLandlord(ctx, domainuser.NewID(landlordUID)).
		Return(draft, nil).Once()
	// Пока заявка в черновике, ответы на ФИО перехватит анкета заявки.
	// Поэтому запускать анкету договора нельзя.
	f.sender.EXPECT().SendMessage(ctx, landlordChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "/list")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(landlordChat, landlordUID, "/data"))
	require.NoError(t, err)
	require.True(t, handled)
	f.parties.AssertNotCalled(t, "SetPartyData", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestDataCommandSendsContractWhenBothReady(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(pairedStub(fullData()), nil).Once()
	f.sender.EXPECT().SendDocument(ctx, tenantChat, "Договор_найма_123456.pdf", mock.Anything).
		Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "/data"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestLastSideToFillGetsContractWithoutAsking(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	// Арендатор вводит телефон последним — договор уходит сразу.
	halfDone := fullData()
	halfDone.TenantPhone = ""

	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(pairedStub(halfDone), nil).Once()
	f.parties.EXPECT().SetPartyData(ctx, listing.ID(1), contract.PartyTenant, mock.MatchedBy(
		func(d listing.ContractData) bool { return d.TenantPhone == "89031234567" },
	)).Return(nil).Once()
	f.sender.EXPECT().SendDocument(ctx, tenantChat, "Договор_найма_123456.pdf", mock.Anything).
		Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "89031234567"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestRedaCommandOverwritesData(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	paired := pairedStub(fullData())

	// /redata просит новое ФИО.
	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(paired, nil).Once()
	f.sender.EXPECT().SendMessage(ctx, tenantChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "новое ФИО")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "/redata"))
	require.NoError(t, err)
	require.True(t, handled)

	// ФИО принято, ждём телефон.
	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(paired, nil).Once()
	f.parties.EXPECT().SetPartyData(ctx, listing.ID(1), contract.PartyTenant, mock.MatchedBy(
		func(d listing.ContractData) bool { return d.TenantFullName == "Сидоров Пётр" },
	)).Return(nil).Once()
	f.sender.EXPECT().SendMessage(ctx, tenantChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "новый телефон")
	})).Return(nil).Once()

	handled, err = f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "Сидоров Пётр"))
	require.NoError(t, err)
	require.True(t, handled)

	// Телефон принят, данные перезаписаны, договор уходит.
	// Новое ФИО уже записано на предыдущем шаге, поэтому заявка его содержит.
	renamed := fullData()
	renamed.TenantFullName = "Сидоров Пётр"
	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(pairedStub(renamed), nil).Once()
	f.parties.EXPECT().SetPartyData(ctx, listing.ID(1), contract.PartyTenant, mock.MatchedBy(
		func(d listing.ContractData) bool {
			return d.TenantFullName == "Сидоров Пётр" && d.TenantPhone == "89039998877"
		},
	)).Return(nil).Once()
	f.sender.EXPECT().SendDocument(ctx, tenantChat, "Договор_найма_123456.pdf", mock.Anything).
		Return(nil).Once()

	handled, err = f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "89039998877"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestRedaRejectsBadNameAndKeepsAsking(t *testing.T) {
	ctx := context.Background()
	f := setup(t)

	paired := pairedStub(fullData())

	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(paired, nil).Once()
	f.sender.EXPECT().SendMessage(ctx, tenantChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "новое ФИО")
	})).Return(nil).Once()

	_, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "/redata"))
	require.NoError(t, err)

	// Код вместо ФИО не принимается, вопрос повторяется.
	f.expectUserGet(tenant())
	f.listings.EXPECT().FindActiveByTenant(ctx, domainuser.NewID(tenantUID)).
		Return(paired, nil).Once()
	f.sender.EXPECT().SendMessage(ctx, tenantChat, mock.MatchedBy(func(s string) bool {
		return strings.Contains(s, "новое ФИО")
	})).Return(nil).Once()

	handled, err := f.handler.HandleUpdate(ctx, message(tenantChat, tenantUID, "123456"))
	require.NoError(t, err)
	require.True(t, handled)
	f.parties.AssertNotCalled(t, "SetPartyData", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
