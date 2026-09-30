ifneq (,$(wildcard ./.env))
    include .env
    export
endif

BUILD_DIR := build
TARGET := app
GO_MAIN := ./cmd/bot/main.go
# Схема pgx5 обязательна: драйвер golang-migrate зарегистрирован под ней,
# а postgres:// — это схема другого драйвера, на lib/pq.
# Приложение при этом ходит к БД через postgres:// (pgx stdlib).
MIGRATE_URL ?= pgx5://maxrent:maxrent@localhost:5432/maxrent?sslmode=disable
MIGRATION_PATH ?= ./migrations
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X main.BuildTime=$(BUILD_TIME)"

.DEFAULT_GOAL := help

.PHONY: help build run test test-race cover lint fmt mockery migrate migrate-down migrate-create migrate-status install-tools up down logs ps clean tidy

help: ## Список доступных команд
	@echo "Доступные команды:"
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Сборка бинаря в build/app
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(TARGET) $(GO_MAIN)
	@echo "Готово: $(BUILD_DIR)/$(TARGET)"

run: ## Запуск приложения без сборки
	go run $(GO_MAIN)

test: ## Запуск тестов
	go test -v -race ./...

cover: ## Покрытие тестами в coverage.out
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1

lint: ## Статический анализ
	golangci-lint run

fmt: ## Форматирование кода
	gofmt -w -s .
	goimports -w . 2>/dev/null || true

mockery: ## Генерация моков для портов
	mockery

migrate: ## Применить миграции
	migrate -path $(MIGRATION_PATH) -database "$(MIGRATE_URL)" up

migrate-down: ## Откатить последнюю миграцию
	migrate -path $(MIGRATION_PATH) -database "$(MIGRATE_URL)" down 1

migrate-create: ## Создать миграцию
	@test -n "$(name)" || (echo "Укажите имя: make migrate-create name=add_users" && exit 1)
	migrate create -ext sql -dir $(MIGRATION_PATH) -seq $(name)

migrate-status: ## Статус миграций
	migrate -path $(MIGRATION_PATH) -database "$(MIGRATE_URL)" version

install-tools: ## Установить CLI миграций с драйвером pgx5
	go install -tags 'pgx5 file' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

clean: ## Удалить артефакты сборки
	rm -rf $(BUILD_DIR) coverage.out
