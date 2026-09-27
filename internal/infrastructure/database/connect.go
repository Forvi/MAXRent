// Package database Инициализация подключения к базе данных.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
)

// Connect Создаёт пул соединений с БД.
func Connect(ctx context.Context, cfg *Config) (*sql.DB, error) {
	return NewPostgres(ctx, cfg)
}

// ConnectMust То же, что Connect, но в случае ошибки логирует её и завершает процесс.
func ConnectMust(ctx context.Context, cfg *Config, logger *slog.Logger) *sql.DB {
	db, err := Connect(ctx, cfg)
	if err != nil {
		logger.Error("failed to init database", "err", err)
		os.Exit(1)
	}
	return db
}

// Close Закрывает пул соединений.
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close postgres: %w", err)
	}
	return nil
}

// CloseMust Закрывает пул соединений, логируя ошибку. Используется при остановке приложения.
func CloseMust(db *sql.DB, logger *slog.Logger) {
	if err := Close(db); err != nil {
		logger.Error("failed to close database", "err", err)
	}
}
