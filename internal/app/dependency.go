package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Forvi/maxrent/internal/document/pdf"
	contracthandlers "github.com/Forvi/maxrent/internal/handlers/contract"
	"github.com/Forvi/maxrent/internal/handlers/info"
	listinghandlers "github.com/Forvi/maxrent/internal/handlers/listing"
	userhandlers "github.com/Forvi/maxrent/internal/handlers/user"
	"github.com/Forvi/maxrent/internal/infrastructure/bot"
	"github.com/Forvi/maxrent/internal/infrastructure/config"
	"github.com/Forvi/maxrent/internal/infrastructure/database"
	"github.com/Forvi/maxrent/internal/infrastructure/logger"
	"github.com/Forvi/maxrent/internal/repositories"
	contractservice "github.com/Forvi/maxrent/internal/services/contract"
	listingservice "github.com/Forvi/maxrent/internal/services/listing"
	userservice "github.com/Forvi/maxrent/internal/services/user"
)

// BuildApp Инициализирует инфраструктуру и фичи.
func BuildApp(ctx context.Context, cfg *config.Config) *App {
	// Логгер
	log := logger.NewLogger(cfg.Logger, cfg.App.Env)
	log.Info("configuration loaded", "env", cfg.App.Env, "log_level", cfg.Logger.Level)

	// Миграции: до первого обращения к данным.
	if cfg.App.AutoMigrate {
		if err := database.MigrateUp(cfg.DB.URL, cfg.DB.MigrationPath, log); err != nil {
			log.Error("failed to apply migrations", "err", err)
			exit(log, err)
		}
	}

	// Клиент бота
	botClient, err := bot.NewClient(ctx, cfg.Bot, log)
	if err != nil {
		log.Error("failed to init bot client", "err", err)
		exit(log, err)
	}

	// База данных
	db := database.ConnectMust(ctx, cfg.DB, log)

	// 5. Фичи: user (регистрация и роль), listing (заявка и подключение), info
	userRepo := repositories.NewUserRepositoryAdapter(db, log)
	userService := userservice.NewService(userRepo, log)

	listingRepo := repositories.NewListingRepositoryAdapter(db, log)
	listingService := listingservice.NewService(listingRepo, log)

	contractPartyRepo := repositories.NewContractPartyAdapter(db, log)
	contractService := contractservice.NewService(contractPartyRepo, pdf.NewGenerator(), log)

	// Порядок важен: обработчик договора забирает обычный текст, когда
	// анкета стороны ещё не заполнена, поэтому он идёт после обработчика
	// заявки. Иначе адрес и цена уходили бы в анкету договора.
	handlers := []bot.Handler{
		userhandlers.NewUserHandler(userService, botClient, log),
		listinghandlers.NewListingHandler(listingService, userService, botClient, log),
		contracthandlers.NewHandler(contractService, listingRepo, userService, botClient, log),
		info.NewInfoHandler(botClient, log),
	}

	// 6. Цикл long polling
	poller := bot.NewPoller(botClient, log, handlers...)

	return &App{
		cfg:             cfg,
		db:              db,
		logger:          log,
		poller:          poller,
		shutdownTimeout: cfg.App.ShutdownTimeout,
	}
}

// exit Логирует ошибку и завершает процесс с ненулевым кодом.
func exit(log *slog.Logger, err error) {
	log.Error("application build failed", "err", err)
	_, _ = fmt.Fprint(os.Stderr, "application build failed: ", err, "\n")
	os.Exit(1)
}
