package user_test

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

	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	userhandler "github.com/Forvi/maxrent/internal/handlers/user"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports/mocks"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

const (
	testChatID  = int64(555)
	testUserID  = int64(777)
	testCBID    = "cb-1"
	testUserDOM = domainuser.PayloadTenant
)

func setup(t *testing.T) (*userhandler.UserHandler, *mocks.MockUserRepository, *mocks.MockMessageSender) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := mocks.NewMockUserRepository(t)
	sender := mocks.NewMockMessageSender(t)
	svc := userservice.NewService(repo, log)

	return userhandler.NewUserHandler(svc, sender, log), repo, sender
}

func startCommand() maxapi.Update {
	return maxapi.Update{
		Type:    maxapi.UpdateMessageCreated,
		ChatID:  testChatID,
		UserID:  testUserID,
		Command: maxapi.Command{Name: "/start"},
	}
}

func TestStartAsksRoleForNewUser(t *testing.T) {
	ctx := context.Background()
	handler, repo, sender := setup(t)
	id := domainuser.NewID(testUserID)

	repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()
	repo.EXPECT().Create(ctx, mock.Anything).Return(nil).Once()

	// Проверяем содержимое клавиатуры, а не только факт вызова:
	// пустая клавиатура раньше проходила незамеченной, и MAX отвечал errors.empty.
	sender.EXPECT().
		SendMessageWithKeyboard(ctx, testChatID, "Кто вы?", mock.MatchedBy(hasRoleButtons)).
		Return(nil).
		Once()

	require.NoError(t, handler.HandleUpdate(ctx, startCommand()))
	repo.AssertExpectations(t)
	sender.AssertExpectations(t)
}

// hasRoleButtons проверяет, что клавиатура содержит обе кнопки выбора роли
// с правильными payload. Содержимое берётся через ToModel, чтобы тест
// ловил потерю кнопок на любом этапе конвертации.
func hasRoleButtons(kb *maxapi.Keyboard) bool {
	if kb == nil || kb.IsEmpty() {
		return false
	}

	built := kb.ToModel().Build()

	want := map[string]string{
		"Арендатор":    domainuser.PayloadTenant,
		"Арендодатель": domainuser.PayloadLandlord,
	}

	found := 0
	for _, row := range built.Payload.Buttons {
		for _, btn := range row {
			if want[btn.Text] == btn.Payload {
				found++
			}
		}
	}

	return found == len(want)
}

func TestStartWithRoleDoesNotAskAgain(t *testing.T) {
	ctx := context.Background()
	handler, repo, sender := setup(t)
	id := domainuser.NewID(testUserID)
	existing := domainuser.New(id, time.Now()).WithRole(domainuser.RoleLandlord)

	repo.EXPECT().FindByID(ctx, id).Return(existing, nil).Once()
	sender.EXPECT().
		SendMessage(ctx, testChatID, mock.MatchedBy(func(text string) bool {
			return strings.Contains(text, "Арендодатель")
		})).
		Return(nil).
		Once()

	require.NoError(t, handler.HandleUpdate(ctx, startCommand()))
	// Кнопки повторно не показываются.
	sender.AssertNotCalled(t, "SendMessageWithKeyboard", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	repo.AssertExpectations(t)
}

func TestStartWithBotSuffix(t *testing.T) {
	ctx := context.Background()
	handler, repo, sender := setup(t)
	id := domainuser.NewID(testUserID)

	repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()
	repo.EXPECT().Create(ctx, mock.Anything).Return(nil).Once()
	sender.EXPECT().
		SendMessageWithKeyboard(ctx, testChatID, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	update := startCommand()
	update.Command.Name = "/start@t320_hakaton_max_bot"

	require.NoError(t, handler.HandleUpdate(ctx, update))
	repo.AssertExpectations(t)
}

func TestForeignCommandIsIgnored(t *testing.T) {
	ctx := context.Background()
	handler, _, sender := setup(t)

	update := startCommand()
	update.Command.Name = "/info"

	// /info обслуживается другой фичей: пользователь не должен получить два ответа.
	require.NoError(t, handler.HandleUpdate(ctx, update))
	sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "SendMessageWithKeyboard", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestPlainTextIsIgnored(t *testing.T) {
	ctx := context.Background()
	handler, _, sender := setup(t)

	require.NoError(t, handler.HandleUpdate(ctx, maxapi.Update{
		Type:    maxapi.UpdateMessageCreated,
		ChatID:  testChatID,
		UserID:  testUserID,
		Text:    "привет",
		Command: maxapi.Command{},
	}))
	sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoleButtonSavesRole(t *testing.T) {
	ctx := context.Background()
	handler, repo, sender := setup(t)
	id := domainuser.NewID(testUserID)

	repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()
	repo.EXPECT().Create(ctx, mock.Anything).Return(nil).Once()
	repo.EXPECT().SetRole(ctx, id, domainuser.RoleTenant).Return(nil).Once()
	// Подсказка следующего шага зависит от роли: арендатор ищет заявку по коду.
	sender.EXPECT().
		AnswerCallback(ctx, testCBID, mock.MatchedBy(func(text string) bool {
			return strings.Contains(text, "код заявки")
		})).
		Return(nil).
		Once()

	require.NoError(t, handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     testChatID,
		UserID:     testUserID,
		Payload:    testUserDOM,
		CallbackID: testCBID,
	}))
	repo.AssertExpectations(t)
	sender.AssertExpectations(t)
}

func TestUnknownRolePayloadIsIgnored(t *testing.T) {
	ctx := context.Background()
	handler, repo, sender := setup(t)

	require.NoError(t, handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     testChatID,
		UserID:     testUserID,
		Payload:    "role_admin",
		CallbackID: testCBID,
	}))
	repo.AssertNotCalled(t, "SetRole", mock.Anything, mock.Anything, mock.Anything)
	sender.AssertNotCalled(t, "AnswerCallback", mock.Anything, mock.Anything, mock.Anything)
}

func TestRoleSaveErrorStillAnswersCallback(t *testing.T) {
	ctx := context.Background()
	handler, repo, sender := setup(t)
	id := domainuser.NewID(testUserID)

	repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()
	repo.EXPECT().Create(ctx, mock.Anything).Return(nil).Once()
	repo.EXPECT().SetRole(ctx, id, domainuser.RoleTenant).Return(errors.New("db is down")).Once()
	// Без ответа на кнопке остался бы индикатор ожидания.
	sender.EXPECT().
		AnswerCallback(ctx, testCBID, mock.Anything).
		Return(nil).
		Once()

	require.NoError(t, handler.HandleUpdate(ctx, maxapi.Update{
		Type:       maxapi.UpdateMessageCallback,
		ChatID:     testChatID,
		UserID:     testUserID,
		Payload:    domainuser.PayloadTenant,
		CallbackID: testCBID,
	}))
	sender.AssertExpectations(t)
}

func TestOtherUpdateTypesIgnored(t *testing.T) {
	ctx := context.Background()
	handler, _, sender := setup(t)

	require.NoError(t, handler.HandleUpdate(ctx, maxapi.Update{
		Type:   maxapi.UpdateBotStarted,
		UserID: testUserID,
	}))
	sender.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
}
