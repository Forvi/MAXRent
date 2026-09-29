package app

import (
	"context"
	"errors"
	"os/signal"
	"syscall"

	"github.com/Forvi/maxrent/internal/infrastructure/database"
)

// Run Запускает приложение и блокируется до сигнала остановки (SIGINT/SIGTERM).
func Run(ctx context.Context, application *App) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- application.start(ctx)
	}()

	select {
	case <-ctx.Done():
		return application.shutdown()
	case err := <-errCh:
		return errors.Join(err, application.shutdown())
	}
}

// start Запускает цикл long polling.
func (a *App) start(ctx context.Context) error {
	a.logger.Info("application started", "polling_timeout", a.cfg.Bot.PollingTimeout)
	return a.poller.Run(ctx)
}

// shutdown Корректно останавливает приложение и освобождает ресурсы.
func (a *App) shutdown() error {
	a.logger.Info("graceful shutdown started", "timeout", a.shutdownTimeout)

	if err := database.Close(a.db); err != nil {
		a.logger.Error("graceful shutdown finished with errors", "err", err)
		return err
	}

	a.logger.Info("graceful shutdown finished")
	return nil
}
