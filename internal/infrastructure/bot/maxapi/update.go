// Package maxapi Изолирует типы библиотеки MAX Bot API от фичей.
// Фичи работают с Update и UpdateType из этого пакета, а не с model.Update вендора,
// поэтому смена библиотеки не затрагивает доменный код.
package maxapi

import "github.com/max-messenger/max-bot-api-client-go/v2/model"

// UpdateType Тип входящего события.
type UpdateType string

// Типы событий, которые бот обрабатывает.
const (
	// UpdateMessageCreated Новое сообщение от пользователя.
	UpdateMessageCreated UpdateType = UpdateType(model.UpdateMessageCreated)
	// UpdateMessageCallback Нажатие на inline-кнопку.
	UpdateMessageCallback UpdateType = UpdateType(model.UpdateMessageCallback)
	// UpdateBotStarted Пользователь начал диалог с ботом.
	UpdateBotStarted UpdateType = UpdateType(model.UpdateBotStarted)
	// UpdateBotRemoved Бот удалён из чата.
	UpdateBotRemoved UpdateType = UpdateType(model.UpdateBotRemoved)
)

// Update Входящее событие в терминах приложения.
type Update struct {
	// Type тип события.
	Type UpdateType
	// ChatID идентификатор чата, из которого пришло событие.
	ChatID int64
	// UserID идентификатор пользователя-отправителя.
	UserID int64
	// MessageID идентификатор сообщения.
	MessageID string
	// Text текст сообщения.
	Text string
	// Command разобранная команда, например /start с аргументами.
	Command Command
	// Payload полезная нагрузка inline-кнопки.
	Payload string
	// CallbackID идентификатор нажатия, нужен для ответа на кнопку.
	CallbackID string
}

// Command Разобранная команда из текста сообщения.
type Command struct {
	// Name имя команды с ведущим слешем, например "/start".
	Name string
	// Params аргументы команды.
	Params []string
	// Text исходный текст команды.
	Text string
}

// IsCommand сообщает, начинается ли текст с команды.
func (c Command) IsCommand() bool {
	return c.Name != ""
}

// FromModelUpdate Конвертирует событие библиотеки в тип приложения.
func FromModelUpdate(u model.Update) Update {
	converted := Update{
		Type:      UpdateType(u.UpdateType),
		ChatID:    u.ChatID,
		UserID:    u.UserID,
		MessageID: u.MessageID,
		Text:      u.GetMessage().Body.Text,
	}

	cmd := u.GetCommand()
	converted.Command = Command{
		Name:   cmd.Command,
		Params: cmd.Params,
		Text:   cmd.RemainingText,
	}

	if u.Callback != nil {
		converted.Payload = u.Callback.Payload
		converted.CallbackID = u.Callback.CallbackID
	}

	return converted
}
