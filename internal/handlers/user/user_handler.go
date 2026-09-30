// Package user Обработчик регистрации пользователя и выбора роли в сделке.
package user

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

// Тексты диалога регистрации.
const (
	askRoleText   = "Кто вы?"
	roleSavedText = "Принято: вы %s. Дальше подскажу, что нужно для оформления договора."
	alreadyKnown  = "Вы уже зарегистрированы как %s. Продолжаем оформление."
	genericError  = "Что-то пошло не так, попробуйте /start ещё раз."
)

// UserHandler Обрабатывает /start и нажатия кнопок выбора роли.
type UserHandler struct {
	service *userservice.Service
	sender  ports.MessageSender
	logger  *slog.Logger
}

// NewUserHandler Создаёт обработчик регистрации пользователя.
func NewUserHandler(
	service *userservice.Service,
	sender ports.MessageSender,
	logger *slog.Logger,
) *UserHandler {
	return &UserHandler{
		service: service,
		sender:  sender,
		logger:  logger,
	}
}

// HandleUpdate Разбирает входящее событие и выполняет нужное действие.
func (h *UserHandler) HandleUpdate(ctx context.Context, update maxapi.Update) error {
	switch update.Type {
	case maxapi.UpdateMessageCallback:
		return h.handleRoleChoice(ctx, update)
	case maxapi.UpdateMessageCreated:
		return h.handleMessage(ctx, update)
	default:
		return nil
	}
}

// handleMessage Обрабатывает текстовые команды. Остальные команды обслуживаются
// другими обработчиками, поэтому здесь игнорируются: иначе пользователь получил бы
// два ответа на одну команду.
func (h *UserHandler) handleMessage(ctx context.Context, update maxapi.Update) error {
	// В группах команда приходит с суффиксом бота: /start@MyBot
	name, _, _ := strings.Cut(update.Command.Name, "@")
	if name != "/start" {
		return nil
	}

	return h.handleStart(ctx, update)
}

// handleStart Регистрирует пользователя и спрашивает роль, если она ещё не выбрана.
func (h *UserHandler) handleStart(ctx context.Context, update maxapi.Update) error {
	registered, err := h.service.Register(ctx, domainuser.NewID(update.UserID))
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to register user", "err", err, "user_id", update.UserID)

		return h.reply(ctx, update.ChatID, genericError)
	}

	if registered.HasRole() {
		return h.reply(ctx, update.ChatID, fmt.Sprintf(alreadyKnown, registered.RoleTitle()))
	}

	return h.askRole(ctx, update.ChatID)
}

// askRole Отправляет вопрос с кнопками выбора роли.
func (h *UserHandler) askRole(ctx context.Context, chatID int64) error {
	kb := maxapi.NewKeyboard()
	kb.AddRow().AddCallbackButton("Арендатор", domainuser.PayloadTenant)
	kb.AddRow().AddCallbackButton("Арендодатель", domainuser.PayloadLandlord)

	if err := h.sender.SendMessageWithKeyboard(ctx, chatID, askRoleText, kb); err != nil {
		h.logger.ErrorContext(ctx, "failed to ask role", "err", err, "chat_id", chatID)

		return err
	}

	return nil
}

// handleRoleChoice Сохраняет роль, выбранную кнопкой.
func (h *UserHandler) handleRoleChoice(ctx context.Context, update maxapi.Update) error {
	role, err := domainuser.RoleFromPayload(update.Payload)
	if err != nil {
		h.logger.WarnContext(ctx, "unknown role payload", "payload", update.Payload, "user_id", update.UserID)

		// Нераспознанный payload — это мусор на стороне клиента, а не сбой приложения.
		// Возвращать ошибку в цикл опроса незачем: апдейт мы всё равно отбросили.
		return nil //nolint:nilerr // намеренно: ошибка разбора payload не является сбоем
	}

	updated, err := h.service.SetRole(ctx, domainuser.NewID(update.UserID), role)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to save role", "err", err, "user_id", update.UserID)

		// Подтверждаем нажатие в любом случае, иначе на кнопке останется индикатор.
		if answerErr := h.sender.AnswerCallback(ctx, update.CallbackID, "Не удалось сохранить выбор, попробуйте ещё раз."); answerErr != nil {
			h.logger.ErrorContext(ctx, "failed to answer callback", "err", answerErr)
		}

		return nil
	}

	confirmation := fmt.Sprintf(roleSavedText, updated.RoleTitle())
	if err := h.sender.AnswerCallback(ctx, update.CallbackID, confirmation); err != nil {
		h.logger.ErrorContext(ctx, "failed to answer callback", "err", err, "user_id", update.UserID)
	}

	return nil
}

// reply Отправляет текстовое сообщение, логируя ошибку отправки.
func (h *UserHandler) reply(ctx context.Context, chatID int64, text string) error {
	if err := h.sender.SendMessage(ctx, chatID, text); err != nil {
		h.logger.ErrorContext(ctx, "failed to send message", "err", err, "chat_id", chatID)

		return nil
	}

	h.logger.InfoContext(ctx, "message sent", "chat_id", chatID)

	return nil
}
