package database

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // драйвер pgx5
	_ "github.com/golang-migrate/migrate/v4/source/file"     // источник из директории
)

// Схемы подключения: приложение ходит к БД через postgres://,
// драйвер golang-migrate зарегистрирован под pgx5://.
const (
	postgresPrefix = "postgres://"
	pgx5Prefix     = "pgx5://"
)

// MigrateUp применяет миграции до последней версии.
// Отсутствие новых миграций ошибкой не считается.
// Приложение открывает собственное соединение: миграции выполняются
// на старте, до первого запроса в базу.
func MigrateUp(dbURL, path string, logger *slog.Logger) error {
	m, err := migrate.New("file://"+path, migrateURL(dbURL))
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	// Close здесь освобождает только соединение самого мигратора.
	defer func() {
		if sourceErr, dbErr := m.Close(); sourceErr != nil || dbErr != nil {
			logger.Warn("failed to close migrator", "source_err", sourceErr, "db_err", dbErr)
		}
	}()

	logger.Info("applying migrations", "path", path)

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("schema is up to date", "version", currentVersion(m))

			return nil
		}

		return fmt.Errorf("apply migrations: %w", err)
	}

	logger.Info("migrations applied", "version", currentVersion(m))

	return nil
}

// currentVersion Возвращает текущую версию схемы для лога.
func currentVersion(m *migrate.Migrate) any {
	version, dirty, err := m.Version()
	if err != nil {
		return "unknown"
	}

	if dirty {
		return fmt.Sprintf("%d (dirty)", version)
	}

	return version
}

// migrateURL Переводит строку подключения приложения в схему драйвера golang-migrate.
func migrateURL(dbURL string) string {
	if strings.HasPrefix(dbURL, pgx5Prefix) {
		return dbURL
	}

	return pgx5Prefix + strings.TrimPrefix(dbURL, postgresPrefix)
}
