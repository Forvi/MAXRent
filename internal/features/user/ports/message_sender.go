package ports

import "context"

// MessageSender Порт отправки сообщений в чат. Реализуется клиентом мессенджера.
//
//go:generate mockery
type MessageSender interface {
	// SendMessage Отправляет текстовое сообщение в указанный чат.
	SendMessage(ctx context.Context, chatID int64, text string) error
}
