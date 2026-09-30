package listing_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/domain/listing"
	"github.com/Forvi/maxrent/internal/ports/mocks"
	listingservice "github.com/Forvi/maxrent/internal/services/listing"
	"github.com/Forvi/maxrent/internal/testutil/listingfixture"
)

var (
	landlordID = listingfixture.LandlordID
	tenantID   = listingfixture.TenantID
)

// maxCodeAttempts Дублирует лимит сервиса: тест проверяет границу,
// поэтому значение обязано совпадать с кодовым.
const maxCodeAttempts = 5

func newService(t *testing.T) (*listingservice.Service, *mocks.MockListingRepository) {
	t.Helper()

	repo := mocks.NewMockListingRepository(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return listingservice.NewService(repo, log), repo
}

func TestStartOrResumeCreatesDraft(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	repo.EXPECT().FindActiveByLandlord(ctx, landlordID).
		Return(listing.Listing{}, listing.ErrNoActiveListing).Once()
	repo.EXPECT().Create(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.LandlordID == landlordID && l.IsDraft() && l.Step == listing.StepAddress
	})).Return(listing.Listing{ID: 1, LandlordID: landlordID, Status: listing.StatusDraft}, nil).Once()

	draft, err := svc.StartOrResume(ctx, landlordID)

	require.NoError(t, err)
	require.Equal(t, listing.ID(1), draft.ID)
	repo.AssertExpectations(t)
}

func TestStartOrResumeKeepsExistingDraft(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)
	existing := listing.Draft(landlordID, time.Now())
	existing.ID = 7

	repo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(existing, nil).Once()

	resumed, err := svc.StartOrResume(ctx, landlordID)

	require.NoError(t, err)
	require.Equal(t, listing.ID(7), resumed.ID, "начатая анкета не должна затираться")
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestAnswerPublishesWhenDone(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	draft := listingfixture.DraftAtStep(t, listing.StepDescription)
	draft.ID = 9

	// Последний ответ публикует заявку: код подбирается и проверяется на занятость.
	repo.EXPECT().CodeTaken(ctx, mock.Anything).Return(false, nil).Once()
	repo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.Status == listing.StatusPublished && l.HasCode() && l.Step == listing.StepDone
	})).Return(nil).Once()

	updated, published, err := svc.Answer(ctx, draft, "Свежий ремонт")

	require.NoError(t, err)
	require.True(t, published)
	require.Equal(t, listing.StatusPublished, updated.Status)
	repo.AssertExpectations(t)
}

func TestAnswerKeepsDraftBetweenSteps(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	draft := listingfixture.DraftAtStep(t, listing.StepAddress)
	draft.ID = 9

	repo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.IsDraft() && l.Step == listing.StepPrice
	})).Return(nil).Once()

	updated, published, err := svc.Answer(ctx, draft, "Москва, ул. Тверская, д. 1, кв. 5")

	require.NoError(t, err)
	require.False(t, published)
	require.Equal(t, listing.StepPrice, updated.Step)
	repo.AssertNotCalled(t, "CodeTaken", mock.Anything)
}

func TestAnswerRetriesOnTakenCode(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	draft := listingfixture.DraftAtStep(t, listing.StepDescription)
	draft.ID = 9

	// Первый код занят, второй свободен.
	repo.EXPECT().CodeTaken(ctx, mock.Anything).Return(true, nil).Once()
	repo.EXPECT().CodeTaken(ctx, mock.Anything).Return(false, nil).Once()
	repo.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()

	updated, published, err := svc.Answer(ctx, draft, "Описание")

	require.NoError(t, err)
	require.True(t, published)
	require.True(t, updated.HasCode())
}

func TestAnswerRejectsInvalidWithoutSaving(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	draft := listingfixture.DraftAtStep(t, listing.StepAddress)
	draft.ID = 9

	_, published, err := svc.Answer(ctx, draft, "Москва")

	require.ErrorIs(t, err, listing.ErrInvalidAnswer)
	require.False(t, published)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestAnswerRejectsFinishedListing(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())

	_, _, err := svc.Answer(ctx, published, "что-то")

	require.ErrorIs(t, err, listing.ErrInvalidAnswer)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestFindByCode(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())
	repo.EXPECT().FindByCode(ctx, listingfixture.Code).Return(published, nil).Once()

	found, err := svc.FindByCode(ctx, tenantID, "123456")

	require.NoError(t, err)
	require.Equal(t, listingfixture.Code, found.Code)
}

func TestFindByCodeRateLimits(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	// Пять неверных кодов подряд: запросы к БД ещё проходят.
	repo.EXPECT().FindByCode(ctx, listing.Code("000000")).
		Return(listing.Listing{}, listing.ErrNotFound).Times(maxCodeAttempts)

	for range maxCodeAttempts {
		_, err := svc.FindByCode(ctx, tenantID, "000000")
		require.ErrorIs(t, err, listing.ErrNotFound)
	}

	// Шестая попытка блокируется до обращения к БД.
	_, err := svc.FindByCode(ctx, tenantID, "000000")
	require.ErrorIs(t, err, listingservice.ErrTooManyAttempts)
	require.True(t, repo.AssertNumberOfCalls(t, "FindByCode", maxCodeAttempts))
}

func TestFindByCodeResetsAttemptsOnSuccess(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())

	// Коды различаем явно: mock.Anything перехватил бы успешный вызов.
	repo.EXPECT().FindByCode(ctx, listing.Code("000000")).
		Return(listing.Listing{}, listing.ErrNotFound).Twice()
	repo.EXPECT().FindByCode(ctx, listingfixture.Code).Return(published, nil).Twice()

	for range 2 {
		_, err := svc.FindByCode(ctx, tenantID, "000000")
		require.ErrorIs(t, err, listing.ErrNotFound)

		found, err := svc.FindByCode(ctx, tenantID, "123456")
		require.NoError(t, err)
		require.Equal(t, listingfixture.Code, found.Code)
	}

	// Успех сбрасывает счётчик: лимит не должен быть исчерпан.
	require.True(t, repo.AssertNumberOfCalls(t, "FindByCode", 4))
}

func TestJoinPairsListing(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())
	repo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.Status == listing.StatusPaired && l.TenantID != nil && *l.TenantID == tenantID
	})).Return(nil).Once()

	paired, err := svc.Join(ctx, tenantID, published)

	require.NoError(t, err)
	require.Equal(t, listing.StatusPaired, paired.Status)
}

func TestJoinRejectsOwnListing(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	published := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now())
	published.LandlordID = tenantID

	_, err := svc.Join(ctx, tenantID, published)

	require.ErrorIs(t, err, listing.ErrSameUser)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestJoinRejectsAlreadyPaired(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	paired := listingfixture.DraftAtStep(t, listing.StepDone).Complete(listingfixture.Code, time.Now()).WithTenant(tenantID, time.Now())

	_, err := svc.Join(ctx, tenantID, paired)

	require.ErrorIs(t, err, listing.ErrAlreadyPaired)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestCancel(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	active := listingfixture.DraftAtStep(t, listing.StepPrice)
	active.ID = 5
	active.Status = listing.StatusPublished

	repo.EXPECT().FindActiveByLandlord(ctx, landlordID).Return(active, nil).Once()
	repo.EXPECT().Update(ctx, mock.MatchedBy(func(l listing.Listing) bool {
		return l.Status == listing.StatusCancelled
	})).Return(nil).Once()

	require.NoError(t, svc.Cancel(ctx, landlordID))
}

func TestCancelWithoutListing(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	repo.EXPECT().FindActiveByLandlord(ctx, landlordID).
		Return(listing.Listing{}, listing.ErrNoActiveListing).Once()

	err := svc.Cancel(ctx, landlordID)

	require.ErrorIs(t, err, listing.ErrNoActiveListing)
}

func TestFreeCodePropagatesRepoError(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	draft := listingfixture.DraftAtStep(t, listing.StepDescription)
	draft.ID = 9

	repo.EXPECT().CodeTaken(ctx, mock.Anything).Return(false, errors.New("db is down")).Once()

	_, _, err := svc.Answer(ctx, draft, "Описание")
	require.Error(t, err)
}
