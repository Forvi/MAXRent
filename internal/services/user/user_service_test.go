package user_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/ports/mocks"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

func newService(t *testing.T) (*userservice.Service, *mocks.MockUserRepository) {
	t.Helper()

	repo := mocks.NewMockUserRepository(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	return userservice.NewService(repo, log), repo
}

func TestRegisterCreatesUser(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)
	id := domainuser.NewID(42)

	repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()
	repo.EXPECT().Create(ctx, mock.MatchedBy(func(u domainuser.User) bool {
		return u.ID == id && !u.HasRole()
	})).Return(nil).Once()

	registered, err := svc.Register(ctx, id)

	require.NoError(t, err)
	require.Equal(t, id, registered.ID)
	require.False(t, registered.HasRole())
	repo.AssertExpectations(t)
}

func TestRegisterKeepsExistingRole(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)
	id := domainuser.NewID(42)
	existing := domainuser.New(id, time.Now()).WithRole(domainuser.RoleLandlord)

	// Повторный /start не должен пересоздавать пользователя и терять роль.
	repo.EXPECT().FindByID(ctx, id).Return(existing, nil).Once()

	registered, err := svc.Register(ctx, id)

	require.NoError(t, err)
	require.True(t, registered.HasRole())
	require.Equal(t, domainuser.RoleLandlord, *registered.Role)
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestRegisterRejectsInvalidID(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)

	registered, err := svc.Register(ctx, domainuser.NewID(0))

	require.Error(t, err)
	require.Equal(t, domainuser.User{}, registered)
	repo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}

func TestSetRole(t *testing.T) {
	ctx := context.Background()
	id := domainuser.NewID(42)

	tests := []struct {
		name     string
		role     domainuser.Role
		wantErr  bool
		wantRole domainuser.Role
	}{
		{name: "tenant", role: domainuser.RoleTenant, wantRole: domainuser.RoleTenant},
		{name: "landlord", role: domainuser.RoleLandlord, wantRole: domainuser.RoleLandlord},
		{name: "empty role", role: domainuser.Role(""), wantErr: true},
		{name: "forged role", role: domainuser.Role("admin"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newService(t)

			// На некорректной роли запросов в репозиторий быть не должно.
			if tt.wantErr {
				_, err := svc.SetRole(ctx, id, tt.role)
				require.Error(t, err)
				require.ErrorIs(t, err, domainuser.ErrUnknownRole)
				repo.AssertNotCalled(t, "SetRole", mock.Anything, mock.Anything, mock.Anything)

				return
			}

			repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()
			repo.EXPECT().Create(ctx, mock.Anything).Return(nil).Once()
			repo.EXPECT().SetRole(ctx, id, tt.wantRole).Return(nil).Once()

			updated, err := svc.SetRole(ctx, id, tt.role)

			require.NoError(t, err)
			require.True(t, updated.HasRole())
			require.Equal(t, tt.wantRole, *updated.Role)
			repo.AssertExpectations(t)
		})
	}
}

func TestRoleFromPayload(t *testing.T) {
	tests := []struct {
		payload string
		want    domainuser.Role
		wantErr bool
	}{
		{payload: domainuser.PayloadTenant, want: domainuser.RoleTenant},
		{payload: domainuser.PayloadLandlord, want: domainuser.RoleLandlord},
		{payload: "role_admin", wantErr: true},
		{payload: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.payload, func(t *testing.T) {
			got, err := domainuser.RoleFromPayload(tt.payload)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, domainuser.ErrUnknownRole)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNewRoleFromStorage(t *testing.T) {
	role, err := domainuser.NewRole("tenant")
	require.NoError(t, err)
	require.Equal(t, domainuser.RoleTenant, role)

	_, err = domainuser.NewRole("nonsense")
	require.ErrorIs(t, err, domainuser.ErrUnknownRole)
}

func TestSetRolePropagatesRepoError(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)
	id := domainuser.NewID(42)
	dbErr := errors.New("connection refused")

	repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()
	repo.EXPECT().Create(ctx, mock.Anything).Return(nil).Once()
	repo.EXPECT().SetRole(ctx, id, domainuser.RoleTenant).Return(dbErr).Once()

	_, err := svc.SetRole(ctx, id, domainuser.RoleTenant)

	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)
}

func TestGetReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	svc, repo := newService(t)
	id := domainuser.NewID(42)

	repo.EXPECT().FindByID(ctx, id).Return(domainuser.User{}, domainuser.ErrNotFound).Once()

	found, err := svc.Get(ctx, id)

	require.ErrorIs(t, err, domainuser.ErrNotFound)
	require.Equal(t, domainuser.User{}, found)
}
