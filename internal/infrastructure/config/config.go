// Package config Загрузка и валидация конфигурации приложения из переменных окружения.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"github.com/Forvi/maxrent/internal/infrastructure/bot"
	"github.com/Forvi/maxrent/internal/infrastructure/database"
	"github.com/Forvi/maxrent/internal/infrastructure/logger"
)

// Config Агрегирует конфигурации всех подсистем приложения.
type Config struct {
	App    *AppConfig
	Bot    *bot.Config
	DB     *database.Config
	Logger *logger.Config
}

// AppConfig Общие настройки приложения.
type AppConfig struct {
	// Env окружение: dev или prod. Единственный источник APP_ENV —
	// логгер берёт его отсюда, а не разбирает переменную повторно.
	Env string `env:"ENV" envDefault:"dev"`
	// ShutdownTimeout таймаут graceful shutdown.
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

// Load Загружает .env (если файл есть) и читает конфигурацию приложения.
func Load() (*Config, error) {
	// Отсутствие .env не является ошибкой: конфигурация может прийти из окружения контейнера.
	_ = godotenv.Load()

	appCfg := &AppConfig{}
	if err := env.ParseWithOptions(appCfg, env.Options{Prefix: "APP_"}); err != nil {
		return nil, fmt.Errorf("load app config: %w", err)
	}

	logCfg, err := logger.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load logger config: %w", err)
	}

	dbCfg, err := database.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load database config: %w", err)
	}

	botCfg, err := bot.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load bot config: %w", err)
	}

	return &Config{
		App:    appCfg,
		Bot:    botCfg,
		DB:     dbCfg,
		Logger: logCfg,
	}, nil
}

// LoadMust То же, что Load, но завершает процесс при ошибке.
func LoadMust() *Config {
	cfg, err := Load()
	if err != nil {
		_, _ = fmt.Fprint(os.Stderr, "failed to load config: ", err, "\n")
		os.Exit(1)
	}
	return cfg
}
