// Package contract Обработчик анкеты сторон и команды /contract.
package contract

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports"
	contractservice "github.com/Forvi/maxrent/internal/services/contract"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

// Тексты диалога.
const (
	askNameText       = "Укажите фамилию, имя и отчество."
	askPhoneText      = "Укажите контактный телефон."
	rewriteNameText   = "Введите новое ФИО — заменю предыдущее."
	rewritePhoneText  = "Введите новый телефон — заменю предыдущий."
	readyText         = "Данные обеих сторон собраны. Формирую договор…"
	draftGuardText    = "Сначала завершите заявку командой /list — договор собирается из неё. Потом пришлите /data."
	noRoleText        = "Сначала выберите роль командой /start."
	noListingLandlord = "Сначала создайте заявку командой /list."
	noListingTenant   = "Вы пока не подключены ни к одной заявке. Отправьте код арендодателя."
	genericError      = "Что-то пошло не так, попробуйте позже."
	sendErrorText     = "Не удалось отправить документ, попробуйте позже."

	// waitingText ждёт данных второй стороны.
	waitingText = "Ваши данные сохранены. Жду данные %s — договор пришлю сразу, " +
		"как только они будут.\n\nКоманда /data — посмотреть статус, /redata — исправить свои."
	// notReadyOtherText: не хватает данных второй стороны, свои уже есть.
	notReadyOtherText = "Договор пока не сформировать: жду данные %s. Ваши данные уже сохранены."
	// notReadySelfText: не хватает данных самой стороны.
	notReadySelfText = "Договор пока не сформировать: заполните свои данные командой /data."

	// dataCommand открывает анкету.
	dataCommand = "/data"
	// redataCommand пересобирает данные стороны заново.
	redataCommand = "/redata"
	// docCommand запускает формирование документа.
	docCommand = "/contract"
)

// rewrite Собирает данные стороны заново: сначала ФИО, потом телефон.
type rewrite struct {
	// nameDone ФИО уже принято, ждём телефон.
	nameDone bool
}

// Handler Ведёт анкету сторон и формирует договор.
type Handler struct {
	contracts *contractservice.Service
	listings  ports.ListingRepository
	users     *userservice.Service
	sender    ports.MessageSender
	logger    *slog.Logger

	// rewrites пользователи, попросившие пересобрать свои данные.
	//
	// Состояние живёт в памяти, а не в базе: перезапись нужна на один
	// диалог из двух ответов, и терять её при перезапуске не страшно —
	// достаточно повторить /redata.
	mu       sync.Mutex
	rewrites map[int64]rewrite
}

// NewHandler Создаёт обработчик договоров.
func NewHandler(
	contracts *contractservice.Service,
	listings ports.ListingRepository,
	users *userservice.Service,
	sender ports.MessageSender,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		contracts: contracts,
		listings:  listings,
		users:     users,
		sender:    sender,
		logger:    logger,
		rewrites:  make(map[int64]rewrite),
	}
}

// armRewrite Запоминает, что пользователь пересобирает свои данные.
func (h *Handler) armRewrite(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.rewrites[userID] = rewrite{}
}

// rewriting Сообщает, идёт ли пересборка данных, и отмечает, принят ли ФИО.
func (h *Handler) rewriting(userID int64) (rewrite, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.rewrites[userID]

	return state, ok
}

// finishRewrite Завершает пересборку данных пользователя.
func (h *Handler) finishRewrite(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.rewrites, userID)
}

// HandleUpdate Ведёт пользователя по сценарию договора.
//
// Возвращает handled: событие обработано здесь или оно не наше. Обработчик
// забирает себе:
//
//   - команды /data и /contract;
//   - обычный текст, когда анкета стороны ещё не заполнена.
//
// Последнее условие важно: обычный текст — это ещё и ответ анкеты заявки.
// Забирая его всегда, мы перехватывали бы адрес и цену у арендодателя, а
// шестизначный код арендатора — сохраняли бы ему в ФИО.
func (h *Handler) HandleUpdate(ctx context.Context, update maxapi.Update) (bool, error) {
	if update.Type != maxapi.UpdateMessageCreated {
		return false, nil
	}

	command := update.CommandName()

	switch command {
	case dataCommand, redataCommand, docCommand:
	case "":
		// Обычный текст: забираем, только если анкета ещё не заполнена.
	default:
		// Остальные команды обслуживают другие обработчики.
		return false, nil
	}

	user, err := h.users.Get(ctx, domainuser.NewID(update.UserID))
	if err != nil {
		// Пользователь ещё не регистрировался: сценарий начнётся после /start.
		return false, nil //nolint:nilerr // отсутствие пользователя не повод прерывать опрос
	}

	if user.Role == nil {
		if command == "" {
			return false, nil
		}

		return true, h.reply(ctx, update.ChatID, noRoleText)
	}

	active, err := h.loadListing(ctx, domainuser.NewID(update.UserID), *user.Role)
	if err != nil {
		if command == "" {
			// Нет заявки — обычный текст не наш.
			return false, nil
		}

		return true, h.explainNoListing(ctx, update, *user.Role, err)
	}

	party := contractservice.PartyFor(*user.Role)

	// Пока заявка в черновике, ответы на ФИО и телефон перехватит обработчик
	// заявки: он отвечает на тот же обычный текст. Без этой проверки
	// анкета договора замирала бы навсегда, а пользователь об этом не узнавал.
	if *user.Role == domainuser.RoleLandlord && active.IsDraft() {
		return true, h.reply(ctx, update.ChatID, draftGuardText)
	}

	switch command {
	case dataCommand:
		return true, h.askNext(ctx, update.ChatID, *user.Role, active)
	case redataCommand:
		// Данные уже введены — исправить их иначе нечем.
		h.armRewrite(update.UserID)

		return true, h.reply(ctx, update.ChatID, rewriteNameText)
	case docCommand:
		return true, h.buildAndSend(ctx, update.ChatID, *user.Role, active)
	default:
		_, rewriting := h.rewriting(update.UserID)
		if !rewriting && active.ContractData.Name(party) != "" && active.ContractData.Phone(party) != "" {
			// Анкета стороны уже заполнена: молчим, чтобы не мешать
			// анкете заявки и не отвечать на каждое сообщение подряд.
			return false, nil
		}

		return true, h.answerNext(ctx, update, active, *user.Role)
	}
}

// loadListing Достаёт заявку, в которой участвует пользователь:
// у арендодателя своя, у арендатора — та, к которой он подключился.
func (h *Handler) loadListing(
	ctx context.Context,
	userID domainuser.ID,
	role domainuser.Role,
) (listing.Listing, error) {
	if role == domainuser.RoleLandlord {
		return h.listings.FindActiveByLandlord(ctx, userID)
	}

	return h.listings.FindActiveByTenant(ctx, userID)
}

// explainNoListing Объясняет, почему заявка недоступна.
func (h *Handler) explainNoListing(
	ctx context.Context,
	update maxapi.Update,
	role domainuser.Role,
	err error,
) error {
	if !errors.Is(err, listing.ErrNoActiveListing) {
		h.logger.ErrorContext(ctx, "failed to load listing", "err", err, "user_id", update.UserID)

		return h.reply(ctx, update.ChatID, genericError)
	}

	if role == domainuser.RoleLandlord {
		return h.reply(ctx, update.ChatID, noListingLandlord)
	}

	return h.reply(ctx, update.ChatID, noListingTenant)
}

// askNext Отвечает по состоянию анкеты стороны.
//
// Раньше здесь был тупик: сторона с полными данными получала
// «введите /data», хотя только что его и вводила, и повторяла по кругу.
// Теперь сообщение называет, чьи данные нужны, и не предлагает бесполезного,
// а когда собраны обе стороны — сразу отправляет договор.
func (h *Handler) askNext(
	ctx context.Context,
	chatID int64,
	role domainuser.Role,
	l listing.Listing,
) error {
	data := l.ContractData
	party := contractservice.PartyFor(role)

	switch {
	case data.Name(party) == "":
		return h.reply(ctx, chatID, askNameText)
	case data.Phone(party) == "":
		return h.reply(ctx, chatID, askPhoneText)
	case data.IsComplete():
		return h.buildAndSend(ctx, chatID, role, l)
	default:
		return h.reply(ctx, chatID, fmt.Sprintf(waitingText, party.Other().TitleGenitive()))
	}
}

// answerNext Применяет введённые данные к анкете стороны.
func (h *Handler) answerNext(ctx context.Context, update maxapi.Update, active listing.Listing, role domainuser.Role) error {
	party := contractservice.PartyFor(role)
	data := active.ContractData
	state, rewriting := h.rewriting(update.UserID)

	// Поля анкеты собираются по одному, поэтому и проверяются по одному.
	switch {
	case rewriting && !state.nameDone:
		name, err := contract.ValidateName(update.Text)
		if err != nil {
			return h.reply(ctx, update.ChatID, rewriteNameText)
		}

		data.SetName(party, name)
		h.rememberNameTaken(update.UserID)

		if err := h.save(ctx, update, active, party, data); err != nil {
			return err
		}

		return h.reply(ctx, update.ChatID, rewritePhoneText)
	case rewriting && state.nameDone:
		phone, err := contract.ValidatePhone(update.Text)
		if err != nil {
			return h.reply(ctx, update.ChatID, rewritePhoneText)
		}

		data.SetPhone(party, phone)
		h.finishRewrite(update.UserID)

		if err := h.save(ctx, update, active, party, data); err != nil {
			return err
		}

		return h.askNext(ctx, update.ChatID, role, active.WithContractData(data))
	case data.Name(party) == "":
		name, err := contract.ValidateName(update.Text)
		if err != nil {
			return h.askNext(ctx, update.ChatID, role, active.WithContractData(data))
		}

		data.SetName(party, name)
	case data.Phone(party) == "":
		phone, err := contract.ValidatePhone(update.Text)
		if err != nil {
			return h.askNext(ctx, update.ChatID, role, active.WithContractData(data))
		}

		data.SetPhone(party, phone)
	default:
		// Анкета уже заполнена: подсказываем следующий шаг.
		return h.askNext(ctx, update.ChatID, role, active.WithContractData(data))
	}

	if err := h.save(ctx, update, active, party, data); err != nil {
		return err
	}

	// Последняя заполненная сторона отправляет договор сама: ждать,
	// пока пользователь вспомнит про /contract, не нужно.
	return h.askNext(ctx, update.ChatID, role, active.WithContractData(data))
}

// rememberNameTaken Отмечает, что при пересборке данных ФИО уже принято.
func (h *Handler) rememberNameTaken(userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state := h.rewrites[userID]
	state.nameDone = true
	h.rewrites[userID] = state
}

// save Записывает сведения стороны в заявку.
func (h *Handler) save(
	ctx context.Context,
	update maxapi.Update,
	active listing.Listing,
	party contract.Party,
	data listing.ContractData,
) error {
	if err := h.contracts.SavePartyData(ctx, active.ID, party, data); err != nil {
		h.logger.ErrorContext(ctx, "failed to save party data", "err", err, "user_id", update.UserID)

		return h.reply(ctx, update.ChatID, genericError)
	}

	return nil
}

// buildAndSend Формирует договор и отправляет его стороне.
//
// Сообщение о неготовности адресуется по роли: нельзя просить ввести /data
// того, кто уже всё ввёл — он застревал в цикле подсказок и не понимал,
// что делать дальше.
func (h *Handler) buildAndSend(
	ctx context.Context,
	chatID int64,
	role domainuser.Role,
	active listing.Listing,
) error {
	if !active.ContractData.IsComplete() {
		missing := missingSide(active.ContractData)

		h.logger.InfoContext(ctx, "contract is not ready",
			"listing_id", active.ID.String(),
			"missing_side", missing,
			"tenant_filled", active.ContractData.HasTenant(),
			"landlord_filled", active.ContractData.HasLandlord(),
		)

		if missing == contractservice.PartyFor(role) {
			return h.reply(ctx, chatID, notReadySelfText)
		}

		return h.reply(ctx, chatID, fmt.Sprintf(notReadyOtherText, missing.TitleGenitive()))
	}

	doc, err := h.contracts.BuildDocument(ctx, active)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to build contract", "err", err)

		return h.reply(ctx, chatID, genericError)
	}

	content, err := h.contracts.Render(doc)
	if err != nil {
		h.logger.ErrorContext(ctx, "failed to render contract", "err", err)

		return h.reply(ctx, chatID, genericError)
	}

	if err := h.sender.SendDocument(ctx, chatID, doc.FileName(), content); err != nil {
		h.logger.ErrorContext(ctx, "failed to send contract", "err", err, "chat_id", chatID)

		return h.reply(ctx, chatID, sendErrorText)
	}

	h.logger.InfoContext(ctx, "contract sent",
		"chat_id", chatID,
		"listing_id", active.ID.String(),
		"bytes", len(content),
	)

	return nil
}

// missingSide Возвращает сторону, чьих данных не хватает.
func missingSide(data listing.ContractData) contract.Party {
	if !data.HasTenant() {
		return contract.PartyTenant
	}

	return contract.PartyLandlord
}

// reply Отправляет текстовое сообщение, логируя ошибку.
func (h *Handler) reply(ctx context.Context, chatID int64, text string) error {
	if err := h.sender.SendMessage(ctx, chatID, text); err != nil {
		h.logger.ErrorContext(ctx, "failed to send message", "err", err, "chat_id", chatID)

		return nil
	}

	return nil
}
