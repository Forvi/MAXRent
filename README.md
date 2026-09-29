# MAXRent

## Стек

| Назначение            | Выбор                                   |
|-----------------------|-----------------------------------------|
| БД                    | PostgreSQL                              |
| Драйвер               | `github.com/jackc/pgx/v5` (+`stdlib`)   |
| Миграции              | `golang-migrate`                        |
| API мессенджера       | `max-messenger/max-bot-api-client-go/v2`|
| Логи                  | `log/slog` + `lmittmann/tint`           |
| Конфигурация          | `caarlos0/env/v11`                      |
| Тесты                 | `stretchr/testify` + `vektra/mockery`   |
| Линтер                | `golangci-lint` v2                      |

## Структура

```
cmd/bot/main.go                     точка входа
internal/
  app/
    app.go                          структура App
    dependency.go                   BuildApp - ручное внедрение зависимостей
    lifecycle.go                    Run - старт, сигналы, graceful shutdown
  infrastructure/
    config/                         конфигурация приложения (.env)
    bot/                            клиент MAX, long polling, порт Handler
      maxapi/                       перевод типов библиотеки в типы приложения
    database/                       подключение к PostgreSQL, пул соединений
    logger/                         slog: tint (dev) / JSON (prod)
  features/<domain>/
    domain/                         сущности, value-objects, доменные ошибки
    dto/                            вход/выходные структуры юзкейсов
    ports/                          интерфейсы-порты + mocks/
    adapters/                       реализации портов
    service/                        юзкейсы + unit-тесты
    handlers/                       обработчики событий бота
migrations/                         *.up.sql / *.down.sql
```

Направление зависимостей: `handlers -> service -> ports <- adapters`.
Фичи знают только про `bot.Handler` и `maxapi.Update`: типы библиотеки MAX
не проникают в доменный код, их конвертация живёт в `bot/maxapi`.

## Быстрый старт

```bash
cp .env.example .env      # при необходимости поправить значения
make up                   # postgres + миграции + бот в docker
```

Локально, без docker:

```bash
docker compose up -d postgres
make migrate              # применить миграции
make run
```

## Команды

```bash
make help            # список команд
make build           # сборка бинаря
make test            # тесты с -race
make cover           # покрытие
make lint            # golangci-lint
make mockery         # перегенерация моков
make migrate         # применить миграции
make migrate-down    # откатить последнюю миграцию
make migrate-create name=add_orders
```

## Переменные окружения

| Переменная              | Default                                   | Описание                        |
|-------------------------|-------------------------------------------|---------------------------------|
| `APP_ENV`               | `dev`                                     | `dev` — цветной лог, `prod` — JSON |
| `APP_LOG_LEVEL`         | `INFO`                                    | `DEBUG`/`INFO`/`WARN`/`ERROR`   |
| `APP_SHUTDOWN_TIMEOUT`  | `15s`                                     | таймаут graceful shutdown        |
| `DB_URL`                | — (обязателен)                            | строка подключения к PostgreSQL  |
| `DB_MAX_OPEN_CONNS`     | `10`                                      | размер пула соединений          |
| `DB_MAX_IDLE_CONNS`     | `5`                                       | простаивающие соединения         |
| `DB_CONN_MAX_LIFETIME`  | `1h`                                      | максимум жизни соединения        |
| `DB_CONN_MAX_IDLE_TIME` | `5m`                                      | максимум простоя соединения      |
| `DB_PING_TIMEOUT`       | `5s`                                      | таймаут проверки БД при старте    |
| `BOT_TOKEN`             | — (обязателен)                            | токен бота от MasterBot          |
| `BOT_REQUEST_TIMEOUT`   | `10s`                                     | таймаут HTTP-запроса к API       |
| `BOT_POLLING_TIMEOUT`   | `30s`                                     | таймаут long polling             |
| `BOT_POLLING_PAUSE`     | `500ms`                                   | пауза между запросами апдейтов   |
| `BOT_ERROR_RETRY_PAUSE` | `5s`                                      | пауза перед повтором при ошибке  |
| `BOT_BASE_URL`          | пусто                                     | адрес API, по умолчанию MAX      |

## Получение токена

Токен выдаёт MasterBot в MAX: откройте диалог с [@MasterBot](https://max.ru/MasterBot)
и следуйте инструкции. Положите токен в `.env` как `BOT_TOKEN=...`.
Без токена приложение не стартует: `BOT_TOKEN` объявлен обязательным, а клиент
дополнительно проверяет его запросом `GetMyInfo` при инициализации.

## Добавление фичи

1. Создать `internal/features/<domain>/` с поддиректориями `domain`, `dto`, `ports`, `adapters`, `service`, `handlers`.
2. Описать порт в `ports/` с директивой `//go:generate mockery`.
3. Реализовать порт в `adapters/` на чистом SQL.
4. Написать юзкейс в `service/` и unit-тест на моке.
5. Выполнить `make mockery`.
6. Реализовать в `handlers/` метод `HandleUpdate(ctx, maxapi.Update) error` — так обработчик
   попадает в цикл long polling.
7. Собрать цепочку адаптер -> юзкейс -> обработчик в `internal/app/dependency.go` и добавить
   обработчик в срез `bot.Handler`.
8. Добавить миграцию и применить `make migrate`.

## Как устроен бот

`internal/infrastructure/bot` — единственное место, где живёт библиотека MAX:

- `bot.NewClient` создаёт API-клиент и проверяет токен вызовом `GetMyInfo`;
- `bot.Poller` выполняет long polling, ведёт `marker` и передаёт апдейты обработчикам;
- `bot.Handler` — порт для фич: `HandleUpdate(ctx, maxapi.Update) error`;
- `bot/maxapi` конвертирует `model.Update` в `maxapi.Update`, поэтому фичи не импортируют
  библиотеку MAX и не зависят от её типов.

Сетевая ошибка при опросе не завершает работу: цикл логирует её, ждёт `BOT_ERROR_RETRY_PAUSE`
и повторяет запрос. Перезапуск контейнера в этом не участвует.
Ошибка одного обработчика не отменяет обработку остальных апдейтов.

Webhook-режим (`Api.GetHandler`) в проекте не используется — бот работает на long polling.

