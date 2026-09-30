// Package bot Реализация клиента мессенджера MAX и цикла long polling.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go/v2"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

// defaultErrorRetryPause Пауза перед повтором запроса, если не задана в конфигурации.
const defaultErrorRetryPause = 5 * time.Second

// Client Обёртка над клиентом MAX Bot API: скрывает библиотеку за своими типами,
// чтобы фичи не зависели от типов вендора.
type Client struct {
	api        *maxbot.Api
	logger     *slog.Logger
	errorRetry time.Duration
}

// pollingMargin Запас поверх таймаута long polling, чтобы HTTP-клиент не оборвал
// запрос раньше, чем API вернёт ответ.
const pollingMargin = 10 * time.Second

// NewClient Создаёт клиента бота и проверяет токен запросом информации о боте.
func NewClient(ctx context.Context, cfg *Config, logger *slog.Logger) (*Client, error) {
	// Один HTTP-клиент обслуживает и обычные запросы, и long polling. Его таймаут
	// обязан быть больше таймаута опроса, иначе запрос апдейтов всегда обрывается
	// и бот не получает сообщения.
	httpTimeout := cfg.RequestTimeout
	if httpTimeout <= cfg.PollingTimeout {
		httpTimeout = cfg.PollingTimeout + pollingMargin
	}

	opts := []maxbot.Opt{
		maxbot.WithHTTPClient(&http.Client{Timeout: httpTimeout}),
		maxbot.WithPollingTimeout(cfg.PollingTimeout),
		maxbot.WithPollingPause(cfg.PollingPause),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, maxbot.WithBaseURL(cfg.BaseURL))
	}

	api, err := maxbot.NewApi(cfg.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("create max api client: %w", err)
	}

	// Токен проверяется сразу при старте, чтобы ошибка была видна в логах на старте,
	// а не через минуту на первом неотвечающем запросе апдейтов.
	info, err := api.Bots.GetMyInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get bot info: %w", err)
	}

	logger.Info("bot authenticated",
		"user_id", info.UserID,
		"username", info.Username,
		"is_bot", info.IsBot,
	)

	return &Client{
		api:        api,
		logger:     logger,
		errorRetry: cfg.ErrorRetryPause,
	}, nil
}

// GetUpdates Запрашивает пачку апдейтов начиная с marker и возвращает новый marker.
func (c *Client) GetUpdates(ctx context.Context, marker int64) ([]maxapi.Update, int64, error) {
	updates, next, err := c.api.Subscriptions.GetUpdates(ctx, marker)
	if err != nil {
		return nil, 0, fmt.Errorf("get updates: %w", err)
	}

	converted := make([]maxapi.Update, 0, len(updates))
	for _, u := range updates {
		converted = append(converted, maxapi.FromModelUpdate(u))
	}

	return converted, next, nil
}

// SendMessage Отправляет текстовое сообщение в чат.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	msg := maxbot.NewMessage().SetChat(chatID).SetText(text)
	if _, err := c.api.Messages.Send(ctx, msg); err != nil {
		return fmt.Errorf("send message to chat %d: %w", chatID, err)
	}
	return nil
}

// SendMessageWithKeyboard Отправляет сообщение с инлайн-клавиатурой.
func (c *Client) SendMessageWithKeyboard(
	ctx context.Context,
	chatID int64,
	text string,
	kb *maxapi.Keyboard,
) error {
	msg := maxbot.NewMessage().SetChat(chatID).SetText(text)
	if kb != nil && !kb.IsEmpty() {
		msg.AddKeyboard(kb.ToModel())
	}

	if _, err := c.api.Messages.Send(ctx, msg); err != nil {
		return fmt.Errorf("send message with keyboard to chat %d: %w", chatID, err)
	}

	return nil
}

// AnswerCallback Подтверждает нажатие на кнопку и убирает индикатор ожидания.
func (c *Client) AnswerCallback(ctx context.Context, callbackID, text string) error {
	answer := model.CallbackAnswer{Message: &model.NewMessageBody{Text: text}}
	if _, err := c.api.Messages.AnswerOnCallback(ctx, callbackID, answer); err != nil {
		return fmt.Errorf("answer callback %s: %w", callbackID, err)
	}

	return nil
}

// errorRetryPause Пауза перед повтором запроса после ошибки.
func (c *Client) errorRetryPause() time.Duration {
	if c.errorRetry <= 0 {
		return defaultErrorRetryPause
	}
	return c.errorRetry
}
