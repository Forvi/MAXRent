# MAXRent

## Стек

| Назначение            | Выбор                                   |
|-----------------------|-----------------------------------------|
| БД                    | PostgreSQL                              |
| Драйвер               | `github.com/jackc/pgx/v5` (+`stdlib`)   |
| Миграции              | `golang-migrate`                        |
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
| `APP_POLLING_INTERVAL`  | `5s`                                      | интервал опроса апдейтов         |
| `APP_SHUTDOWN_TIMEOUT`  | `15s`                                     | таймаут graceful shutdown        |
| `DB_URL`                | — (обязателен)                            | строка подключения к PostgreSQL  |
| `DB_MAX_OPEN_CONNS`     | `10`                                      | размер пула соединений          |
| `DB_MAX_IDLE_CONNS`     | `5`                                       | простаивающие соединения         |
| `DB_CONN_MAX_LIFETIME`  | `1h`                                      | максимум жизни соединения        |
| `DB_CONN_MAX_IDLE_TIME` | `5m`                                      | максимум простоя соединения      |
| `DB_PING_TIMEOUT`       | `5s`                                      | таймаут проверки БД при старте    |

