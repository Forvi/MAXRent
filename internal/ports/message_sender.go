// Package ports Общие порты приложения: контракты, которые реализует инфраструктура.
package ports

import (
	"context"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

// MessageSender Отправка сообщений в чат. Реализуется клиентом мессенджера.
type MessageSender interface {
	// SendMessage Отправляет текстовое сообщение в указанный чат.
	SendMessage(ctx context.Context, chatID int64, text string) error
	// SendMessageWithKeyboard Отправляет сообщение с инлайн-клавиатурой.
	SendMessageWithKeyboard(ctx context.Context, chatID int64, text string, kb *maxapi.Keyboard) error
	// AnswerCallback Подтверждает нажатие на кнопку, убирая индикатор ожидания.
	AnswerCallback(ctx context.Context, callbackID, text string) error
}
