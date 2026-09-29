package handlers_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/features/user/handlers"
	"github.com/Forvi/maxrent/internal/features/user/ports/mocks"
	"github.com/Forvi/maxrent/internal/features/user/service"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

func TestHandleUpdate(t *testing.T) {
	ctx := context.Background()
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name      string
		update    maxapi.Update
		sendErr   error
		wantSend  bool
		wantReply string
	}{
		{
			name: "start command triggers greeting",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				ChatID:  100,
				UserID:  200,
				Command: maxapi.Command{Name: "/start"},
			},
			wantSend:  true,
			wantReply: "Hello",
		},
		{
			name: "start command with bot suffix triggers greeting",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				ChatID:  100,
				Command: maxapi.Command{Name: "/start@t320_hakaton_max_bot"},
			},
			wantSend:  true,
			wantReply: "Hello",
		},
		{
			name: "plain text is ignored",
			update: maxapi.Update{
				Type: maxapi.UpdateMessageCreated,
				Text: "привет",
			},
			wantSend: false,
		},
		{
			name: "unknown command is ignored",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				Command: maxapi.Command{Name: "/unknown"},
			},
			wantSend: false,
		},
		{
			name: "non message update is ignored",
			update: maxapi.Update{
				Type:    maxapi.UpdateBotStarted,
				Command: maxapi.Command{Name: "/start"},
			},
			wantSend: false,
		},
		{
			name: "send error is logged and swallowed",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				ChatID:  100,
				Command: maxapi.Command{Name: "/start"},
			},
			sendErr:   errors.New("api is down"),
			wantSend:  true,
			wantReply: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := mocks.NewMockMessageSender(t)

			if tt.wantSend {
				sender.EXPECT().
					SendMessage(mock.Anything, tt.update.ChatID, tt.wantReply).
					Return(tt.sendErr).
					Once()
			}

			handler := handlers.NewUserHandler(service.NewCreateUser(nil, testLogger), sender, testLogger)

			require.NoError(t, handler.HandleUpdate(ctx, tt.update))
			sender.AssertExpectations(t)
		})
	}
}
