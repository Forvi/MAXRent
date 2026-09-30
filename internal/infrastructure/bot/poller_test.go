package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

// stubHandler Заглушка обработчика: запоминает, сколько раз к нему пришли.
type stubHandler struct {
	// handled что возвращает обработчик.
	handled bool
	// err ошибка, которую он возвращает.
	err error
	// calls сколько раз к нему обратились.
	calls int
}

func (s *stubHandler) HandleUpdate(_ context.Context, _ maxapi.Update) (bool, error) {
	s.calls++

	return s.handled, s.err
}

// newTestRouter Собирает маршрутизатор: проверяем только распределение событий.
func newTestRouter(handlers ...Handler) *Router {
	return NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), handlers...)
}

func TestRouteStopsAtFirstHandlerThatClaimsUpdate(t *testing.T) {
	first := &stubHandler{handled: false}
	second := &stubHandler{handled: true}
	third := &stubHandler{handled: true}

	r := newTestRouter(first, second, third)
	r.Route(context.Background(), maxapi.Update{Type: maxapi.UpdateMessageCreated, Text: "Москва"})

	require.Equal(t, 1, first.calls, "первый должен увидеть событие и отказаться")
	require.Equal(t, 1, second.calls, "второй забрал событие")
	require.Equal(t, 0, third.calls, "третий не должен получать уже обработанное событие")
}

func TestRouteIgnoresUpdateNobodyClaims(t *testing.T) {
	first := &stubHandler{handled: false}
	second := &stubHandler{handled: false}

	r := newTestRouter(first, second)
	r.Route(context.Background(), maxapi.Update{Type: maxapi.UpdateMessageCreated, Text: "привет"})

	require.Equal(t, 1, first.calls)
	require.Equal(t, 1, second.calls, "событие проверяют все, но никто не отвечает")
}

func TestRouteContinuesAfterHandlerError(t *testing.T) {
	// Ошибка отдельного обработчика не должна останавливать очередь:
	// иначе одно сбойное событие заблокировало бы все остальные.
	failing := &stubHandler{handled: true, err: errors.New("send failed")}
	healthy := &stubHandler{handled: true}

	r := newTestRouter(failing, healthy)
	r.Route(context.Background(), maxapi.Update{Type: maxapi.UpdateMessageCreated, Text: "/list"})

	require.Equal(t, 1, failing.calls)
	require.Equal(t, 0, healthy.calls, "обработавший событие останавливает цепочку")
}

func TestRouteWithoutHandlersIsSafe(t *testing.T) {
	r := newTestRouter()
	require.NotPanics(t, func() {
		r.Route(context.Background(), maxapi.Update{Type: maxapi.UpdateMessageCreated})
	})
}
