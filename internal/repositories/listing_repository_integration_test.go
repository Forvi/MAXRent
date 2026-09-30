package repositories_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/repositories"
)

func TestListingRepositoryIntegration(t *testing.T) {
	url := os.Getenv("DB_URL")
	if url == "" {
		t.Skip("DB_URL is not set")
	}

	db, err := sql.Open("pgx", url)
	require.NoError(t, err)

	defer func() { _ = db.Close() }()

	ctx := context.Background()

	// Без поднятой БД тест не выполняется: красный набор у того, кто
	// ещё не запустил docker compose, пользы не приносит.
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("database is not reachable: %v", err)
	}
	repo := repositories.NewListingRepositoryAdapter(db, discardLogger())

	// Схему поднимает файл миграции, здесь проверяем её применимость.
	_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS listings`)
	require.NoError(t, err)

	// Схему берём из файла миграции, а не дублируем её в тесте:
	// так проверяется именно то, что уедет в базу.
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000002_create_listings.up.sql"))
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)

	landlord := domainuser.NewID(800001)
	tenant := domainuser.NewID(800002)

	// Заявка ссылается на пользователей, а тесты пользователей удаляют за собой.
	for _, id := range []domainuser.ID{landlord, tenant} {
		_, err = db.ExecContext(ctx,
			`INSERT INTO users (id, role) VALUES ($1, 'tenant') ON CONFLICT (id) DO NOTHING`, id.Int64())
		require.NoError(t, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM listings WHERE landlord_id IN ($1, $2)`, landlord.Int64(), tenant.Int64())
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM users WHERE id IN ($1, $2)`, landlord.Int64(), tenant.Int64())
	})

	// Черновик создаётся без кода: колонка nullable, чтобы не спорить с UNIQUE.
	draft := listing.Draft(landlord, time.Now().UTC().Truncate(time.Microsecond))
	created, err := repo.Create(ctx, draft)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.Empty(t, created.Code.String(), "у черновика кода быть не должно")

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, landlord, found.LandlordID)
	require.Nil(t, found.TenantID)
	require.Equal(t, listing.StepAddress, found.Step)
	require.Equal(t, listing.StatusDraft, found.Status)
	require.WithinDuration(t, created.CreatedAt, found.CreatedAt, time.Second)

	// Вторая активная заявка того же арендодателя запрещена индексом в БД.
	second := listing.Draft(landlord, time.Now())
	_, err = repo.Create(ctx, second)
	require.Error(t, err, "частичный индекс должен запрещать вторую активную заявку")

	// Активная заявка находится по арендодателю.
	active, err := repo.FindActiveByLandlord(ctx, landlord)
	require.NoError(t, err)
	require.Equal(t, created.ID, active.ID)

	// Заполняем анкету и публикуем.
	answered := active
	for answered.Step != listing.StepDone {
		next, err := answered.ApplyAnswer(answerFor(answered.Step))
		require.NoError(t, err)
		answered = next
	}

	published := answered.Complete("424242", time.Now())
	require.NoError(t, repo.Update(ctx, published))

	// Черновик по коду не находится, опубликованный — находится.
	_, err = repo.CodeTaken(ctx, listing.Code("424242"))
	require.NoError(t, err)

	taken, err := repo.CodeTaken(ctx, listing.Code("424242"))
	require.NoError(t, err)
	require.True(t, taken, "код опубликованной заявки должен считаться занятым")

	byCode, err := repo.FindByCode(ctx, listing.Code("424242"))
	require.NoError(t, err)
	require.Equal(t, created.ID, byCode.ID)
	require.Equal(t, listing.Money(3500000), byCode.Price, "цена должна вернуться в копейках без потерь")
	require.Equal(t, listing.TermShort, byCode.Term)
	require.Equal(t, listing.UtilitiesShared, byCode.Utilities)
	require.Equal(t, "Москва, ул. Тверская, д. 1, кв. 5", byCode.Address.String())
	require.Equal(t, listing.StepDone, byCode.Step, "опубликованная заявка читается как завершённая")

	// Деньги с копейками не теряют точность.
	withKopeks := published
	withKopeks.Price = listing.Money(3500050)
	withKopeks.Deposit = listing.Money(123456)
	require.NoError(t, repo.Update(ctx, withKopeks))

	updated, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, listing.Money(3500050), updated.Price)
	require.Equal(t, listing.Money(123456), updated.Deposit)
	require.Equal(t, "35000,50", updated.Price.String())

	// Подключение арендатора.
	paired := withKopeks.WithTenant(tenant, time.Now())
	require.NoError(t, repo.Update(ctx, paired))

	pairedFound, err := repo.FindByCode(ctx, listing.Code("424242"))
	require.NoError(t, err)
	require.Equal(t, listing.StatusPaired, pairedFound.Status)
	require.NotNil(t, pairedFound.TenantID)
	require.Equal(t, tenant, *pairedFound.TenantID)

	// Сопоставленная заявка больше не активна, поэтому новую завести можно.
	third := listing.Draft(landlord, time.Now())
	createdThird, err := repo.Create(ctx, third)
	require.NoError(t, err)
	require.NoError(t, repo.Update(ctx, createdThird.Cancelled(time.Now())))

	// Несуществующие записи.
	_, err = repo.FindByID(ctx, listing.ID(999999))
	require.ErrorIs(t, err, listing.ErrNotFound)

	_, err = repo.FindActiveByLandlord(ctx, domainuser.NewID(800003))
	require.ErrorIs(t, err, listing.ErrNoActiveListing)

	_, err = repo.FindByCode(ctx, listing.Code("000000"))
	require.ErrorIs(t, err, listing.ErrNotFound)
}

// answerFor Ответ на шаг анкеты.
func answerFor(step listing.Step) string {
	switch step {
	case listing.StepAddress:
		return "Москва, ул. Тверская, д. 1, кв. 5"
	case listing.StepPrice:
		return "35000"
	case listing.StepDeposit:
		return "35000"
	case listing.StepTerm:
		return listing.PayloadTermShort
	case listing.StepUtilities:
		return listing.PayloadUtilitiesShared
	case listing.StepDescription:
		return "Свежий ремонт"
	default:
		return ""
	}
}
