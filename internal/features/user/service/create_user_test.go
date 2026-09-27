package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/features/user/domain"
	"github.com/Forvi/maxrent/internal/features/user/dto"
	"github.com/Forvi/maxrent/internal/features/user/ports/mocks"
	"github.com/Forvi/maxrent/internal/features/user/service"
)

func TestCreateUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name      string
		input     dto.CreateUserInput
		setupMock func(*mocks.MockUserWriter)
		wantErr   error
	}{
		{
			name:  "successfully created",
			input: dto.CreateUserInput{Username: "nikita"},
			setupMock: func(w *mocks.MockUserWriter) {
				w.EXPECT().
					CreateUser(mock.Anything, mock.AnythingOfType("domain.User")).
					Return(domain.User{ID: userID, Username: "nikita"}, nil).
					Once()
			},
			wantErr: nil,
		},
		{
			name:      "validation error",
			input:     dto.CreateUserInput{Username: "x"},
			setupMock: func(w *mocks.MockUserWriter) {},
			wantErr:   domain.ErrUsernameLength,
		},
		{
			name:  "user already exists",
			input: dto.CreateUserInput{Username: "nikita"},
			setupMock: func(w *mocks.MockUserWriter) {
				w.EXPECT().
					CreateUser(mock.Anything, mock.AnythingOfType("domain.User")).
					Return(domain.User{}, domain.ErrAlreadyExists).
					Once()
			},
			wantErr: domain.ErrAlreadyExists,
		},
		{
			name:  "repository failure is wrapped",
			input: dto.CreateUserInput{Username: "nikita"},
			setupMock: func(w *mocks.MockUserWriter) {
				w.EXPECT().
					CreateUser(mock.Anything, mock.AnythingOfType("domain.User")).
					Return(domain.User{}, errors.New("connection refused")).
					Once()
			},
			wantErr: service.ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := mocks.NewMockUserWriter(t)
			tt.setupMock(writer)

			createUser := service.NewCreateUser(writer, testLogger)
			user, err := createUser.Execute(ctx, tt.input)

			require.ErrorIs(t, err, tt.wantErr)

			if tt.wantErr != nil {
				assert.Empty(t, user)
				return
			}

			assert.Equal(t, userID, user.ID)
			assert.Equal(t, tt.input.Username, user.Username)
			writer.AssertExpectations(t)
		})
	}
}
