package logger

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config Конфигурация логгера.
type Config struct {
	// Level уровень логирования: DEBUG, INFO, WARN, ERROR.
	Level string `env:"LOG_LEVEL" envDefault:"INFO"`
}

// LoadConfig читает конфигурацию логгера из переменных окружения с префиксом APP_.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.ParseWithOptions(cfg, env.Options{Prefix: "APP_"}); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return cfg, nil
}
