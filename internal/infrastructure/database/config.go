package database

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config Конфигурация подключения к БД.
type Config struct {
	// URL строка подключения.
	// notEmpty вместо required: required пропускает пустое значение (DB_URL=).
	URL string `env:"URL,notEmpty"`
	// MaxOpenConns максимум открытых соединений в пуле.
	MaxOpenConns int `env:"MAX_OPEN_CONNS" envDefault:"10"`
	// MaxIdleConns максимум простаивающих соединений в пуле.
	MaxIdleConns int `env:"MAX_IDLE_CONNS" envDefault:"5"`
	// ConnMaxLifetime максимальное время жизни соединения.
	ConnMaxLifetime time.Duration `env:"CONN_MAX_LIFETIME" envDefault:"1h"`
	// ConnMaxIdleTime максимальное время простоя соединения в пуле.
	ConnMaxIdleTime time.Duration `env:"CONN_MAX_IDLE_TIME" envDefault:"5m"`
	// PingTimeout таймаут проверки доступности БД при старте.
	PingTimeout time.Duration `env:"PING_TIMEOUT" envDefault:"5s"`
	// MigrationPath каталог с файлами миграций.
	MigrationPath string `env:"MIGRATION_PATH" envDefault:"./migrations"`
}

// LoadConfig читает конфигурацию БД из переменных окружения с префиксом DB_.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.ParseWithOptions(cfg, env.Options{Prefix: "DB_"}); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return cfg, nil
}
