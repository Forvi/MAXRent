// Package ports Общие порты приложения: контракты, которые реализует инфраструктура.
package ports

import "context"

// MessageSender Отправка сообщений в чат. Реализуется клиентом мессенджера.
type MessageSender interface {
	// SendMessage Отправляет текстовое сообщение в указанный чат.
	SendMessage(ctx context.Context, chatID int64, text string) error
}
