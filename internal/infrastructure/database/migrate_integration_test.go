package database_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/infrastructure/database"
)

// scratchDBName База, которую тест создаёт и удаляет: основная база
// разработчика не должна страдать от проверки миграций с нуля.
const scratchDBName = "maxrent_migrate_test"

// TestMigrateUpFromScratch Проверяет главный сценарий развёртывания:
// пустая база, после старта приложения схема полная, повторный запуск безопасен.
func TestMigrateUpFromScratch(t *testing.T) {
	adminURL := os.Getenv("DB_URL")
	if adminURL == "" {
		t.Skip("DB_URL is not set")
	}

	admin, err := sql.Open("pgx", adminURL)
	require.NoError(t, err)

	// Cleanup выполняется в порядке LIFO, поэтому соединение закроется
	// последним — после удаления временной базы.
	t.Cleanup(func() { _ = admin.Close() })

	ctx := t.Context()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	migrations := filepath.Join("..", "..", "..", "migrations")

	dropScratch(t, admin)

	_, err = admin.ExecContext(ctx, `CREATE DATABASE `+scratchDBName)
	require.NoError(t, err)

	t.Cleanup(func() { dropScratch(t, admin) })

	scratchURL := replaceDatabase(adminURL, scratchDBName)

	// Первый запуск на пустой базе применяет все миграции.
	require.NoError(t, database.MigrateUp(scratchURL, migrations, log))
	requireTables(t, scratchURL, "users", "listings")

	// Повторный запуск не должен ни падать, ни ломать схему.
	require.NoError(t, database.MigrateUp(scratchURL, migrations, log))
	requireTables(t, scratchURL, "users", "listings")

	// Версия схмы дошла до последней миграции.
	version := scalar(t, scratchURL, `SELECT version FROM schema_migrations`)
	require.Equal(t, "2", version)

	dirty := scalar(t, scratchURL, `SELECT dirty FROM schema_migrations`)
	require.Equal(t, "false", dirty, "схема не должна остаться в состоянии dirty")
}

// TestMigrateUpMissingPath Фейл должен быть внятным, а не паником.
func TestMigrateUpMissingPath(t *testing.T) {
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("DB_URL is not set")
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := database.MigrateUp(url, filepath.Join("..", "..", "..", "no-such-dir"), log)

	require.Error(t, err)
}

// requireTables Проверяет, что таблицы созданы.
func requireTables(t *testing.T, url string, tables ...string) {
	t.Helper()

	for _, table := range tables {
		query := `SELECT to_regclass('` + table + `') IS NOT NULL`
		require.Equal(t, "true", scalar(t, url, query), "таблица %s должна существовать", table)
	}
}

// scalar Выполняет запрос, возвращающий одно значение.
func scalar(t *testing.T, url, query string) string {
	t.Helper()

	db, err := sql.Open("pgx", url)
	require.NoError(t, err)

	defer func() { _ = db.Close() }()

	var out string
	require.NoError(t, db.QueryRowContext(t.Context(), query).Scan(&out))

	return out
}

// dropScratch Удаляет временную базу, если она осталась с прошлого раза.
// Контекст здесь свой: контекст теста к моменту cleanup уже отменён.
func dropScratch(t *testing.T, admin *sql.DB) {
	t.Helper()

	_, err := admin.ExecContext(context.Background(),
		`DROP DATABASE IF EXISTS `+scratchDBName+` WITH (FORCE)`)
	require.NoError(t, err)
}

// replaceDatabase Заменяет только имя базы, сохраняя хост и параметры подключения.
func replaceDatabase(rawURL, name string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	parsed.Path = "/" + name

	return parsed.String()
}
