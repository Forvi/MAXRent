package app

import (
	"context"

	"github.com/Forvi/maxrent/internal/infrastructure/config"
	"github.com/Forvi/maxrent/internal/infrastructure/database"
	"github.com/Forvi/maxrent/internal/infrastructure/logger"
	"github.com/Forvi/maxrent/internal/infrastructure/polling"
)

// BuildApp Явно инициализирует инфраструктуру и фичи.
// Каждая зависимость создаётся здесь и передаётся дальше по цепочке.
func BuildApp(ctx context.Context, cfg *config.Config) *App {
	// Logger
	log := logger.NewLogger(cfg.Logger, cfg.App.Env)
	log.Info("configuration loaded", "env", cfg.App.Env, "log_level", cfg.Logger.Level)

	// DB
	db := database.ConnectMust(ctx, cfg.DB, log)

	// Features
	//	userRepo := useradapters.NewUserRepository(db, log)
	//	createUser := userservice.NewCreateUser(userRepo, log)
	//	userHandler := userhandlers.NewUserHandler(createUser, log)

	// Цикл опроса апдейтов (заглушка, далее - клиент мессенджера)
	poller := polling.NewPoller(cfg.App.PollingInterval, db, log)

	return &App{
		cfg:             cfg,
		db:              db,
		logger:          log,
		poller:          poller,
		shutdownTimeout: cfg.App.ShutdownTimeout,
	}
}
