package repositories_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/repositories"
)

func TestUserRepositoryIntegration(t *testing.T) {
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("DB_URL is not set")
	}

	db, err := sql.Open("pgx", url)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Приводим схему к актуальной: старая таблица от удалённой заглушки мешает.
	_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS users`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		CREATE TABLE users (
			id         BIGINT PRIMARY KEY,
			role       VARCHAR(16),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_users_role CHECK (role IS NULL OR role IN ('tenant', 'landlord'))
		)`)
	require.NoError(t, err)

	repo := repositories.NewUserRepositoryAdapter(db, discardLogger())
	id := domainuser.NewID(900001)

	// Создание
	created := domainuser.New(id, time.Now().UTC().Truncate(time.Microsecond))
	require.NoError(t, repo.Create(ctx, created))

	found, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, found.ID)
	require.False(t, found.HasRole(), "после регистрации роли быть не должно")
	require.WithinDuration(t, created.CreatedAt, found.CreatedAt, time.Second)

	// Повторная регистрация не должна затирать роль
	require.NoError(t, repo.SetRole(ctx, id, domainuser.RoleLandlord))
	require.NoError(t, repo.Create(ctx, domainuser.New(id, time.Now())))

	afterReCreate, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	require.True(t, afterReCreate.HasRole(), "повторная регистрация не должна сбрасывать роль")
	require.Equal(t, domainuser.RoleLandlord, *afterReCreate.Role)

	// Смена роли
	require.NoError(t, repo.SetRole(ctx, id, domainuser.RoleTenant))
	afterUpdate, err := repo.FindByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, domainuser.RoleTenant, *afterUpdate.Role)

	// Несуществующий пользователь
	_, err = repo.FindByID(ctx, domainuser.NewID(900002))
	require.ErrorIs(t, err, domainuser.ErrNotFound)

	require.ErrorIs(t, repo.SetRole(ctx, domainuser.NewID(900002), domainuser.RoleTenant), domainuser.ErrNotFound)

	// Ограничение на уровне БД: чужая роль не пишется
	_, err = db.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, id.Int64())
	require.Error(t, err, "CHECK-ограничение должно отклонить значение")

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, id.Int64())
	})
}

// discardLogger Логгер, не печатающий ничего: интеграционный тест не должен шуметь.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
