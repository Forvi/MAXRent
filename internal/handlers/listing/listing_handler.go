// Package listing Обработчик заявок: анкета арендодателя и подключение арендатора по коду.
package listing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports"
	listingservice "github.com/Forvi/maxrent/internal/services/listing"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

// Тексты диалога.
const (
	startLandlordText = "Заполним заявку. Отвечайте по шагам, я буду спрашивать."
	startTenantText   = "Введите код заявки арендодателя — 6 цифр."
	publishedText     = "Заявка готова и ждёт арендатора.\n\nВаш код: %s\n\nОтдайте его арендатору — по коду он подключится."
	cancelledText     = "Заявка отменена. Создать новую можно командой /list."
	codeNotFoundText  = "Заявка с таким кодом не найдена. Проверьте цифры и попробуйте ещё раз."
	codeLimitText     = "Слишком много попыток. Подождите немного и попробуйте позже."
	noRoleText        = "Сначала выберите роль командой /start."
	noActiveText      = "У вас нет активной заявки. Создайте командой /list."
	genericErrorText  = "Что-то пошло не так, попробуйте позже."
	alreadyPairedText = "По этому коду уже идёт подбор арендатора."
	pairFoundText     = "Арендатор нашёлся! Заявка собрана, начинаем оформление договора."
	roleMismatchText  = "Эта кнопка доступна только арендатору."
	joinPrefix        = "join_"
)

// ListingHandler Ведёт обе ветки: арендодатель заполняет анкету,
// арендатор подключается по коду.
type ListingHandler struct {
	listings *listingservice.Service
	users    *userservice.Service
	sender   ports.MessageSender
	logger   *slog.Logger
}

// NewListingHandler Создаёт обработчик заявок.
func NewListingHandler(
	listings *listingservice.Service,
	users *userservice.Service,
	sender ports.MessageSender,
	logger *slog.Logger,
) *ListingHandler {
	return &ListingHandler{
		listings: listings,
		users:    users,
		sender:   sender,
		logger:   logger,
	}
}

// HandleUpdate Разбирает событие и ведёт пользователя по его сценарию.
func (h *ListingHandler) HandleUpdate(ctx context.Context, update maxapi.Update) error {
	switch update.Type {
	case maxapi.UpdateMessageCallback:
		return h.handleCallback(ctx, update)
	case maxapi.UpdateMessageCreated:
		return h.handleMessage(ctx, update)
	default:
		return nil
	}
}

// handleMessage Обрабатывает команды и ответы анкеты.
func (h *ListingHandler) handleMessage(ctx context.Context, update maxapi.Update) error {
	command := extractCommand(update.Text)

	// Команду /start обслуживает обработчик user: отвечаем только на свои.
	switch command {
	case "", "/list":
	case "/cancel", "/cancel_list":
	default:
		return nil
	}

	user, err := h.users.Get(ctx, domainuser.NewID(update.UserID))
	if err != nil {
		// Пользователь ещё не регистрировался: сценарий начнётся после /start.
		return nil //nolint:nilerr // отсутствие пользователя не повод прерывать опрос
	}

	if user.Role == nil {
		if command == "" {
			return nil
		}

		return h.reply(ctx, update.ChatID, noRoleText)
	}

	switch *user.Role {
	case domainuser.RoleLandlord:
		return h.handleLandlordMessage(ctx, update, command)
	case domainuser.RoleTenant:
		return h.handleTenantMessage(ctx, update)
	default:
		return nil
	}
}

// handleLandlordMessage Запускает или продолжает анкету арендодателя.
func (h *ListingHandler) handleLandlordMessage(ctx context.Context, update maxapi.Update, command string) error {
	landlordID := domainuser.NewID(update.UserID)

	if command == "/cancel" || command == "/cancel_list" {
		return h.cancel(ctx, update.ChatID, landlordID)
	}

	active, err := h.listings.StartOrResume(ctx, landlordID)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to open listing", "err", err, "user_id", update.UserID)

		return h.reply(ctx, update.ChatID, genericErrorText)
	}

	// Опубликованную заявку заново не спрашиваем: показываем её код.
	if !active.IsDraft() {
		return h.reply(ctx, update.ChatID, fmt.Sprintf(publishedText, active.Code))
	}

	// Команда /list просто продолжает текущий вопрос анкеты.
	if command == "/list" {
		return h.askCurrentStep(ctx, update.ChatID, active)
	}

	return h.answerStep(ctx, update.ChatID, active, update.Text)
}

// handleTenantMessage Принимает код заявки.
func (h *ListingHandler) handleTenantMessage(ctx context.Context, update maxapi.Update) error {
	code, ok := extractCode(update.Text)
	if !ok {
		return h.reply(ctx, update.ChatID, startTenantText)
	}

	found, err := h.listings.FindByCode(ctx, domainuser.NewID(update.UserID), code)
	if err != nil {
		switch {
		case errors.Is(err, listing.ErrNotFound):
			return h.reply(ctx, update.ChatID, codeNotFoundText)
		case errors.Is(err, listingservice.ErrTooManyAttempts):
			return h.reply(ctx, update.ChatID, codeLimitText)
		case errors.Is(err, listing.ErrInvalidAnswer):
			return h.reply(ctx, update.ChatID, startTenantText)
		default:
			h.logger.ErrorContext(ctx, "failed to search listing", "err", err, "user_id", update.UserID)

			return h.reply(ctx, update.ChatID, genericErrorText)
		}
	}

	return h.showPreview(ctx, update.ChatID, found)
}

// handleCallback Обрабатывает нажатия кнопок анкеты и подтверждения подключения.
func (h *ListingHandler) handleCallback(ctx context.Context, update maxapi.Update) error {
	code, isJoin := strings.CutPrefix(update.Payload, joinPrefix)

	switch {
	case isJoin:
		return h.handleJoin(ctx, update, code)
	case isAnswerPayload(update.Payload):
		return h.handleAnswerButton(ctx, update)
	default:
		return nil
	}
}

// hasRole Проверяет роль пользователя. Без проверки арендатор, нажав кнопку
// срока аренды, завёл бы заявку как арендодатель.
func (h *ListingHandler) hasRole(ctx context.Context, userID int64, want domainuser.Role) bool {
	user, err := h.users.Get(ctx, domainuser.NewID(userID))
	if err != nil {
		h.logger.WarnContext(ctx, "cannot verify role", "err", err, "user_id", userID)

		return false
	}

	return user.Role != nil && *user.Role == want
}

// handleAnswerButton Применяет ответ, выбранный кнопкой.
func (h *ListingHandler) handleAnswerButton(ctx context.Context, update maxapi.Update) error {
	if !h.hasRole(ctx, update.UserID, domainuser.RoleLandlord) {
		return h.answerCallback(ctx, update.CallbackID, roleMismatchText)
	}

	landlordID := domainuser.NewID(update.UserID)

	active, err := h.listings.StartOrResume(ctx, landlordID)
	if err != nil {
		return h.answerCallback(ctx, update.CallbackID, genericErrorText)
	}

	if !active.IsDraft() {
		return h.answerCallback(ctx, update.CallbackID, "Анкета уже заполнена.")
	}

	updated, published, err := h.listings.Answer(ctx, active, update.Payload)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to apply answer", "err", err, "user_id", update.UserID)

		return h.answerCallback(ctx, update.CallbackID, "Не получилось принять ответ, попробуйте ещё раз.")
	}

	if published {
		return h.answerCallback(ctx, update.CallbackID, fmt.Sprintf(publishedText, updated.Code))
	}

	// Следующий вопрос уходит отдельным сообщением, а нажатие только подтверждается.
	if err := h.answerCallback(ctx, update.CallbackID, "Принято"); err != nil {
		return err
	}

	return h.askCurrentStep(ctx, update.ChatID, updated)
}

// handleJoin Подключает арендатора к заявке по нажатой кнопке.
func (h *ListingHandler) handleJoin(ctx context.Context, update maxapi.Update, code string) error {
	if !h.hasRole(ctx, update.UserID, domainuser.RoleTenant) {
		return h.answerCallback(ctx, update.CallbackID, roleMismatchText)
	}

	tenantID := domainuser.NewID(update.UserID)

	found, err := h.listings.FindByCode(ctx, tenantID, code)
	if err != nil {
		if errors.Is(err, listing.ErrNotFound) {
			return h.answerCallback(ctx, update.CallbackID, codeNotFoundText)
		}

		if errors.Is(err, listingservice.ErrTooManyAttempts) {
			return h.answerCallback(ctx, update.CallbackID, codeLimitText)
		}

		if errors.Is(err, listing.ErrInvalidAnswer) {
			return h.answerCallback(ctx, update.CallbackID, startTenantText)
		}

		h.logger.ErrorContext(ctx, "failed to find listing before join", "err", err, "user_id", update.UserID)

		return h.answerCallback(ctx, update.CallbackID, genericErrorText)
	}

	if found.Status != listing.StatusPublished {
		return h.answerCallback(ctx, update.CallbackID, alreadyPairedText)
	}

	if _, err := h.listings.Join(ctx, tenantID, found); err != nil {
		if errors.Is(err, listing.ErrSameUser) {
			return h.answerCallback(ctx, update.CallbackID, "Это ваша собственная заявка.")
		}

		if errors.Is(err, listing.ErrAlreadyPaired) {
			return h.answerCallback(ctx, update.CallbackID, alreadyPairedText)
		}

		h.logger.ErrorContext(ctx, "failed to join listing", "err", err, "user_id", update.UserID)

		return h.answerCallback(ctx, update.CallbackID, genericErrorText)
	}

	return h.answerCallback(ctx, update.CallbackID, "Подключено! "+pairFoundText)
}

// answerStep Применяет текстовый ответ к текущему шагу анкеты.
func (h *ListingHandler) answerStep(ctx context.Context, chatID int64, l listing.Listing, answer string) error {
	updated, published, err := h.listings.Answer(ctx, l, answer)
	if err != nil {
		if errors.Is(err, listing.ErrInvalidAnswer) {
			// Ответ не подошёл: повторяем тот же вопрос.
			return h.askCurrentStep(ctx, chatID, l)
		}

		h.logger.ErrorContext(ctx, "failed to save answer", "err", err, "listing_id", l.ID.String())

		return h.reply(ctx, chatID, genericErrorText)
	}

	if published {
		return h.reply(ctx, chatID, fmt.Sprintf(publishedText, updated.Code))
	}

	return h.askCurrentStep(ctx, chatID, updated)
}

// askCurrentStep Отправляет вопрос текущего шага анкеты.
func (h *ListingHandler) askCurrentStep(ctx context.Context, chatID int64, l listing.Listing) error {
	step := l.Step

	if !step.UsesButtons() {
		return h.reply(ctx, chatID, step.Question())
	}

	kb := maxapi.NewKeyboard()

	switch step {
	case listing.StepTerm:
		kb.AddRow().AddCallbackButton("До года", listing.PayloadTermShort)
		kb.AddRow().AddCallbackButton("От года и больше", listing.PayloadTermLong)
	case listing.StepUtilities:
		kb.AddRow().AddCallbackButton("Оплачивает арендатор", listing.PayloadUtilitiesTenant)
		kb.AddRow().AddCallbackButton("Оплачивает арендодатель", listing.PayloadUtilitiesLandlord)
		kb.AddRow().AddCallbackButton("Оплачиваем поровну", listing.PayloadUtilitiesShared)
	}

	return h.sendKeyboard(ctx, chatID, step.Question(), kb)
}

// cancel Отменяет активную заявку арендодателя.
func (h *ListingHandler) cancel(ctx context.Context, chatID int64, landlordID domainuser.ID) error {
	if err := h.listings.Cancel(ctx, landlordID); err != nil {
		if errors.Is(err, listing.ErrNoActiveListing) {
			return h.reply(ctx, chatID, noActiveText)
		}

		h.logger.ErrorContext(ctx, "failed to cancel listing", "err", err, "user_id", landlordID.String())

		return h.reply(ctx, chatID, genericErrorText)
	}

	return h.reply(ctx, chatID, cancelledText)
}

// showPreview Показывает арендатору сводку заявки до подключения.
func (h *ListingHandler) showPreview(ctx context.Context, chatID int64, l listing.Listing) error {
	deposit := "без залога"
	if l.Deposit > 0 {
		deposit = l.Deposit.String() + " ₽"
	}

	summary := fmt.Sprintf(
		"Заявка на аренду:\n\n%s\nАренда: %s ₽/мес\nЗалог: %s\nСрок: %s\nКоммунальные: %s",
		l.Address, l.Price, deposit, l.Term.Title(), l.Utilities.Title(),
	)

	kb := maxapi.NewKeyboard()
	kb.AddRow().AddCallbackButton("Подключиться", joinPrefix+l.Code.String())

	return h.sendKeyboard(ctx, chatID, summary, kb)
}

// answerCallback Подтверждает нажатие на кнопку.
func (h *ListingHandler) answerCallback(ctx context.Context, callbackID, text string) error {
	if callbackID == "" {
		return nil
	}

	if err := h.sender.AnswerCallback(ctx, callbackID, text); err != nil {
		h.logger.ErrorContext(ctx, "failed to answer callback", "err", err)
	}

	return nil
}

// sendKeyboard Отправляет сообщение с клавиатурой, логируя ошибку.
func (h *ListingHandler) sendKeyboard(ctx context.Context, chatID int64, text string, kb *maxapi.Keyboard) error {
	if err := h.sender.SendMessageWithKeyboard(ctx, chatID, text, kb); err != nil {
		h.logger.ErrorContext(ctx, "failed to send message with keyboard", "err", err, "chat_id", chatID)

		return err
	}

	return nil
}

// reply Отправляет текстовое сообщение, логируя ошибку отправки.
func (h *ListingHandler) reply(ctx context.Context, chatID int64, text string) error {
	if err := h.sender.SendMessage(ctx, chatID, text); err != nil {
		h.logger.ErrorContext(ctx, "failed to send message", "err", err, "chat_id", chatID)

		return nil
	}

	return nil
}

// extractCommand Возвращает команду из текста без слеша и суффикса бота.
func extractCommand(text string) string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return ""
	}

	first, _, _ := strings.Cut(trimmed, " ")
	command, _, _ := strings.Cut(first, "@")

	return strings.ToLower(command)
}

// extractCode Извлекает шестизначный код из текста вида «123456» или «/join 123456».
func extractCode(text string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return "", false
	}

	candidate := fields[len(fields)-1]
	if len(candidate) != listing.CodeLength {
		return "", false
	}

	for _, r := range candidate {
		if r < '0' || r > '9' {
			return "", false
		}
	}

	return candidate, true
}

// isAnswerPayload Сообщает, что payload относится к вопросам анкеты.
func isAnswerPayload(payload string) bool {
	return strings.HasPrefix(payload, "term_") || strings.HasPrefix(payload, "utilities_")
}
