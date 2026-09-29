package bot

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config Конфигурация бота.
type Config struct {
	// Token токен бота. Обязателен.
	Token string `env:"TOKEN,notEmpty"`
	// BaseURL адрес API. Пустое значение — использовать адрес MAX по умолчанию.
	BaseURL string `env:"BASE_URL" envDefault:""`
	// RequestTimeout таймаут HTTP-запроса к API.
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" envDefault:"10s"`
	// PollingTimeout таймаут long polling: сколько ждать апдейтов перед пустым ответом.
	PollingTimeout time.Duration `env:"POLLING_TIMEOUT" envDefault:"30s"`
	// PollingPause пауза между запросами апдейтов.
	PollingPause time.Duration `env:"POLLING_PAUSE" envDefault:"500ms"`
	// ErrorRetryPause пауза перед повтором после сетевой ошибки.
	ErrorRetryPause time.Duration `env:"ERROR_RETRY_PAUSE" envDefault:"5s"`
}

// LoadConfig читает конфигурацию бота из переменных окружения с префиксом BOT_.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.ParseWithOptions(cfg, env.Options{Prefix: "BOT_"}); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return cfg, nil
}
