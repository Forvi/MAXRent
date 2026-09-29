package info_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/handlers/info"
	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
	"github.com/Forvi/maxrent/internal/ports/mocks"
)

func TestHandleUpdate(t *testing.T) {
	ctx := context.Background()
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name     string
		update   maxapi.Update
		sendErr  error
		wantSend bool
	}{
		{
			name: "info command triggers answer",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				ChatID:  100,
				UserID:  200,
				Command: maxapi.Command{Name: "/info"},
			},
			wantSend: true,
		},
		{
			name: "info command with bot suffix triggers answer",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				ChatID:  100,
				Command: maxapi.Command{Name: "/info@t320_hakaton_max_bot"},
			},
			wantSend: true,
		},
		{
			name: "start command is ignored",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				Command: maxapi.Command{Name: "/start"},
			},
			wantSend: false,
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
			name: "non message update is ignored",
			update: maxapi.Update{
				Type:    maxapi.UpdateBotStarted,
				Command: maxapi.Command{Name: "/info"},
			},
			wantSend: false,
		},
		{
			name: "send error does not break polling",
			update: maxapi.Update{
				Type:    maxapi.UpdateMessageCreated,
				ChatID:  100,
				Command: maxapi.Command{Name: "/info"},
			},
			sendErr:  errors.New("api is down"),
			wantSend: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := mocks.NewMockMessageSender(t)

			if tt.wantSend {
				// Текст ответа проверяем по содержанию, а не дословно:
				// иначе тест ломается при любой правке формулировки.
				isInfoText := mock.MatchedBy(func(text string) bool {
					return strings.Contains(text, "MAXRent") && strings.Contains(text, "/info")
				})

				sender.EXPECT().
					SendMessage(mock.Anything, tt.update.ChatID, isInfoText).
					Return(tt.sendErr).
					Once()
			}

			handler := info.NewInfoHandler(sender, testLogger)

			require.NoError(t, handler.HandleUpdate(ctx, tt.update))
			sender.AssertExpectations(t)
		})
	}
}
