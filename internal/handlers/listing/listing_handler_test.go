package listing_test

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

	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	listinghandler "github.com/Forvi/maxrent/internal/handlers/listing"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports/mocks"
	listingservice "github.com/Forvi/maxrent/internal/services/listing"
	userservice "github.com/Forvi/maxrent/internal/services/user"
	"github.com/Forvi/maxrent/internal/testutil/listingfixture"
)

const (
	landlordChatID = int64(10)
	tenantChatID   = int64(20)
	callbackID     = "cb-1"
)

// Идентификаторы пользователей для сценариев арендодателя и арендатора.
var (
	landlordID = listingfixture.LandlordID
	tenantID   = listingfixture.TenantID
)

func setup(t *testing.T) (
	*listinghandler.ListingHandler,
	*mocks.MockListingRepository,
	*mocks.MockUserRepository,
	*mocks.MockMessageSender,
) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	listingRepo := mocks.NewMockListingRepository(t)
	userRepo := mocks.NewMockUserRepository(t)
	sender := mocks.NewMockMessageSender(t)

	listingSvc := listingservice.NewService(listingRepo, log)
	userSvc := userservice.NewService(userRepo, log)

	return listinghandler.NewListingHandler(listingSvc, userSvc, sender, log), listingRepo, userRepo, sender
}

func landlord(id domainuser.ID) domainuser.User {
	return domainuser.New(id, time.Now()).WithRole(domainuser.RoleLandlord)
}

func tenant(id domainuser.ID) domainuser.User {
	return domainuser.New(id, time.Now()).WithRole(domainuser.RoleTenant)
}

func expectUserGet(userRepo *mocks.MockUserRepository, u domainuser.User) {
	userRepo.EXPECT().FindByID(mock.Anything, u.ID).Return(u, nil).Once()
}

func textUpdate(chatID, userID int64, text string) maxapi.Update {
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

// commandUpdate Событие с командой. Чата всегда один — landlordChatID.
func commandUpdate(userID int64, command string) maxapi.Update {
	return maxapi.Update{
		Type:    maxapi.UpdateMessageCreated,
		ChatID:  landlordChatID,
		UserID:  userID,
		Text:    command,
		Command: maxapi.Command{Name: command},
	}
}

func TestListCommandAsksFirstQuestion(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(listingfixture.Draft(), nil).Once()
	sender.EXPECT().SendMessage(ctx, landlordChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "адрес")
	})).Return(nil).Once()

	update := commandUpdate(int64(landlordID), "/list")
	handled, err := handler.HandleUpdate(ctx, update)
	require.NoError(t, err)
	require.True(t, handled)
}

func TestLandlordAnswerAdvancesStep(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	draft := listingfixture.Draft()

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(draft, nil).Once()
	listingRepo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.Step == listing.StepPrice
	})).Return(nil).Once()
	sender.EXPECT().SendMessage(ctx, landlordChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "аренда в месяц")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx,
		textUpdate(landlordChatID, int64(landlordID), "Москва, ул. Тверская, д. 1, кв. 5"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestLandlordInvalidAnswerRepeatsQuestion(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(listingfixture.Draft(), nil).Once()
	// Ответ не сохранён, вопрос задан повторно.
	listingRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	sender.EXPECT().SendMessage(ctx, landlordChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "адрес")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, textUpdate(landlordChatID, int64(landlordID), "Москва"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestLandlordTermButtonAsksNextQuestion(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(listingfixture.DraftAtStep(t, listing.StepTerm), nil).Once()
	listingRepo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.Term == listing.TermShort && l.Step == listing.StepUtilities
	})).Return(nil).Once()

	sender.EXPECT().AnswerCallback(ctx, callbackID, "Принято").Return(nil).Once()
	sender.EXPECT().SendMessageWithKeyboard(ctx, landlordChatID, mock.Anything, mock.Anything).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     landlordChatID,
		UserID:     int64(landlordID),
		Payload:    listing.PayloadTermShort,
		CallbackID: callbackID,
	})
	require.NoError(t, err)
	require.True(t, handled)
}

func TestLandlordUtilitiesButtonCompletesListing(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).
		Return(listingfixture.DraftAtStep(t, listing.StepDescription), nil).Once()
	listingRepo.EXPECT().CodeTaken(ctx, mock.Anything).Return(false, nil).Once()
	listingRepo.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()

	sender.EXPECT().AnswerCallback(ctx, callbackID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "код")
	})).Return(nil).Once()

	// Ответ кнопкой на последний шаг завершает анкету.
	handled, err := handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     landlordChatID,
		UserID:     int64(landlordID),
		Payload:    listing.PayloadUtilitiesShared,
		CallbackID: callbackID,
	})
	require.NoError(t, err)
	require.True(t, handled)
}

func TestPublishedListingDoesNotRestartQuestionnaire(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(published, nil).Once()
	sender.EXPECT().SendMessage(ctx, landlordChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "123456")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, commandUpdate(int64(landlordID), "/list"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestCancelListing(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(listingfixture.DraftAtStep(t, listing.StepPrice), nil).Once()
	listingRepo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.Status == listing.StatusCancelled
	})).Return(nil).Once()
	sender.EXPECT().SendMessage(ctx, landlordChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "отменена")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, commandUpdate(int64(landlordID), "/cancel"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestTenantSeesListingPreview(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())

	expectUserGet(userRepo, tenant(tenantID))
	// Арендатор ещё не подключён — значит, код нужно искать.
	listingRepo.EXPECT().FindActiveByTenant(ctx, tenantID).
		Return(listing.Listing{}, listing.ErrNoActiveListing).Once()
	listingRepo.EXPECT().FindByCode(ctx, listingfixture.Code).Return(published, nil).Once()
	// Сводка с кнопкой подтверждения: подключение только после согласия.
	sender.EXPECT().SendMessageWithKeyboard(ctx, tenantChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "Тверская") && strings.Contains(text, "35000")
	}), mock.MatchedBy(hasJoinButton)).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, textUpdate(tenantChatID, int64(tenantID), "123456"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestTenantUnknownCode(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)

	expectUserGet(userRepo, tenant(tenantID))
	listingRepo.EXPECT().FindActiveByTenant(ctx, tenantID).
		Return(listing.Listing{}, listing.ErrNoActiveListing).Once()
	listingRepo.EXPECT().FindByCode(ctx, listing.Code("000000")).
		Return(listing.Listing{}, listing.ErrNotFound).Once()
	sender.EXPECT().SendMessage(ctx, tenantChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "не найдена")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, textUpdate(tenantChatID, int64(tenantID), "000000"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestTenantJoinButtonPairsListing(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())

	expectUserGet(userRepo, tenant(tenantID))
	listingRepo.EXPECT().FindByCode(ctx, listingfixture.Code).Return(published, nil).Once()
	listingRepo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.Status == listing.StatusPaired && l.TenantID != nil
	})).Return(nil).Once()
	sender.EXPECT().AnswerCallback(ctx, callbackID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "Подключено")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     tenantChatID,
		UserID:     int64(tenantID),
		Payload:    "join_123456",
		CallbackID: callbackID,
	})
	require.NoError(t, err)
	require.True(t, handled)
}

func TestTenantCannotJoinOwnListing(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())
	published.LandlordID = tenantID

	expectUserGet(userRepo, tenant(tenantID))
	listingRepo.EXPECT().FindByCode(ctx, listingfixture.Code).Return(published, nil).Once()
	sender.EXPECT().AnswerCallback(ctx, callbackID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "ваша")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     tenantChatID,
		UserID:     int64(tenantID),
		Payload:    "join_123456",
		CallbackID: callbackID,
	})
	require.NoError(t, err)
	require.True(t, handled)
}

func TestForeignCommandIsIgnored(t *testing.T) {
	ctx := context.Background()
	handler, _, _, sender := setup(t)

	// /info обслуживается другой фичей: второй ответ пользователю не нужен.
	handled, err := handler.HandleUpdate(ctx, commandUpdate(int64(landlordID), "/info"))
	require.NoError(t, err)
	require.False(t, handled)
	sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageWithKeyboard", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestUnregisteredUserIsIgnored(t *testing.T) {
	ctx := context.Background()
	handler, _, userRepo, sender := setup(t)

	userRepo.EXPECT().FindByID(mock.Anything, mock.Anything).
		Return(domainuser.User{}, domainuser.ErrNotFound).Once()

	handled, err := handler.HandleUpdate(ctx, textUpdate(landlordChatID, 999, "/list"))
	require.NoError(t, err)
	require.True(t, handled)
	sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestUserWithoutRoleGetsHint(t *testing.T) {
	ctx := context.Background()
	handler, _, userRepo, sender := setup(t)
	noRole := domainuser.New(landlordID, time.Now())

	expectUserGet(userRepo, noRole)
	sender.EXPECT().SendMessage(ctx, landlordChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "/start")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, commandUpdate(int64(landlordID), "/list"))
	require.NoError(t, err)
	require.True(t, handled)
}

func TestTenantPlainTextGoesToContractQuestionnaire(t *testing.T) {
	ctx := context.Background()
	handler, _, userRepo, sender := setup(t)
	renter := tenant(tenantID)

	expectUserGet(userRepo, renter)

	// Текст без кода — это ответ анкеты договора, а не заявки.
	// Обработчик заявки обязан его пропустить, иначе ФИО арендатора
	// перехватывалось бы поиском кода.
	handled, err := handler.HandleUpdate(ctx, textUpdate(tenantChatID, int64(tenantID), "привет"))
	require.NoError(t, err)
	require.False(t, handled)
	sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestRepoErrorDoesNotBreakPolling(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)
	owner := landlord(landlordID)

	expectUserGet(userRepo, owner)
	listingRepo.EXPECT().FindActiveByLandlord(ctx, landlordID).
		Return(listing.Listing{}, errors.New("db is down")).Once()
	sender.EXPECT().SendMessage(ctx, landlordChatID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "позже")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, commandUpdate(int64(landlordID), "/list"))
	require.NoError(t, err)
	require.True(t, handled)
}

// hasJoinButton Проверяет, что в клавиатуре есть кнопка подключения с кодом.
func hasJoinButton(kb *maxapi.Keyboard) bool {
	if kb == nil || kb.IsEmpty() {
		return false
	}

	for _, row := range kb.ToModel().Build().Payload.Buttons {
		for _, btn := range row {
			if btn.Payload == "join_"+listingfixture.Code.String() {
				return true
			}
		}
	}

	return false
}

func TestLandlordButtonRejectedForTenant(t *testing.T) {
	ctx := context.Background()
	handler, listingRepo, userRepo, sender := setup(t)

	// Арендатор не должен завести заявку, нажав кнопку срока аренды.
	expectUserGet(userRepo, tenant(tenantID))
	sender.EXPECT().AnswerCallback(ctx, callbackID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "только арендатору")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     tenantChatID,
		UserID:     int64(tenantID),
		Payload:    listing.PayloadTermShort,
		CallbackID: callbackID,
	})
	require.NoError(t, err)
	require.True(t, handled)
	listingRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	listingRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestJoinButtonRejectedForLandlord(t *testing.T) {
	ctx := context.Background()
	handler, _, userRepo, sender := setup(t)

	expectUserGet(userRepo, landlord(landlordID))
	sender.EXPECT().AnswerCallback(ctx, callbackID, mock.MatchedBy(func(text string) bool {
		return strings.Contains(text, "только арендатору")
	})).Return(nil).Once()

	handled, err := handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     landlordChatID,
		UserID:     int64(landlordID),
		Payload:    "join_123456",
		CallbackID: callbackID,
	})
	require.NoError(t, err)
	require.True(t, handled)
}
