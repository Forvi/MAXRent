# MAXRent

## Стек

| Назначение            | Выбор                                   |
|-----------------------|-----------------------------------------|
| БД                    | PostgreSQL                              |
| Драйвер               | `github.com/jackc/pgx/v5` (+`stdlib`)   |
| Миграции              | `golang-migrate`                        |
| PDF                   | `go-pdf/fpdf` + встроенный DejaVu Sans   |
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
cp .env.example .env      # вписать BOT_TOKEN
make up                   # postgres + бот в docker
```

Миграции бот накатывает сам при старте — отдельный сервис не нужен.
Если нужна чистая база с нуля:

```bash
docker compose down -v   # удалить volume с базой
make up
```

Локально, без сборки контейнера с ботом:

```bash
docker compose up -d postgres
make run                  # миграции применятся автоматически
```

## Миграции

Файлы лежат в `migrations/`, применяются при старте приложения
(`APP_AUTO_MIGRATE=true` по умолчанию). Перед первым обращением к данным
последовательно выполняются `*.up.sql`.

На проде с несколькими репликами автозапуск выключают
(`APP_AUTO_MIGRATE=false`) — иначе миграции будут гнать все инстансы сразу.

Для ручных операций нужен CLI. Он собирается с тегами драйверов, иначе
не будет работать ни одна схема подключения:

```bash
make install-tools       # go install -tags 'pgx5 file' .../cmd/migrate@latest
make migrate-status
make migrate-down
make migrate-create name=add_orders
```

Схема подключения для CLI — `pgx5://` (драйвер на pgx). Приложение при этом
ходит к базе через `postgres://`, это разные схемы для разных драйверов.

### Сертификаты

API MAX работает по TLS с цепочкой от российского удостоверяющего центра
(`Russian Trusted Sub CA`). В стандартном бандле alpine этого корня нет,
поэтому без него бот падает при старте с `x509: certificate signed by
unknown authority` и уходит в бесконечный рестарт — снаружи это выглядит
как «контейнер запустился, но не отвечает».

Корень лежит в `deploy/certs/russian_trusted_root.crt` и ставится в образ
через `update-ca-certificates`. Промежуточный Sub CA в образ не нужен: он
приходит в цепочке с сервера.

На локальной машине для `go run` нужен системный сертификат
(`ca-certificates-russian` в Arch/CachyOS).

## Команды

```bash
make help            # список команд
make build           # сборка бинаря
make test            # тесты с -race
make cover           # покрытие
make lint            # golangci-lint
make mockery         # перегенерация моков
make install-tools   # CLI миграций с драйвером pgx5 (нужен один раз)
make migrate-status  # текущая версия схемы
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
| `DB_MIGRATION_PATH`     | `./migrations`                            | каталог файлов миграций          |
| `APP_AUTO_MIGRATE`      | `true`                                    | накатывать миграции при старте   |
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

## Договор

Команда `/data` запускает анкету: бот спрашивает ФИО и телефон для вашей роли.
Повторный `/data` показывает статус — свои данные сохранены, ждём данные второй
стороны. `/redata` пересобирает ваши данные с нуля, если нужно исправить опечатку.
`/contract` формирует и отправляет PDF; договор уходит тому, кто его запросил,
а также той стороне, которая заполнила данные последней.

В договоре заполняются адрес объекта, аренда, залог, срок и коммунальные — из заявки,
и ФИО с телефонами — из анкеты сторон. Остальные поля (паспортные данные, площади,
основания права, неустойка, подписи) остаются прочерками: в бумажном договоре их
тоже заполняют от руки при подписании.

Текст основан на публичной форме договора найма жилого помещения
(domashniy-urist.ru) и не проходил юридическую проверку.

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
8. Добавить миграцию в `migrations/` — она применится при следующем старте.

## Как устроен бот

`internal/infrastructure/bot` — единственное место, где живёт библиотека MAX:

- `bot.NewClient` создаёт API-клиент и проверяет токен вызовом `GetMyInfo`;
- `bot.Poller` выполняет long polling и ведёт `marker`;
- `bot.Router` раздаёт апдейт первому обработчику, который признал его своим;
- `bot.Handler` — порт для фич: `HandleUpdate(ctx, maxapi.Update) (handled bool, err error)`;
- `bot/maxapi` конвертирует `model.Update` в `maxapi.Update`, поэтому фичи не импортируют
  библиотеку MAX и не зависят от её типов.

Сетевая ошибка при опросе не завершает работу: цикл логирует её, ждёт `BOT_ERROR_RETRY_PAUSE`
и повторяет запрос. Перезапуск контейнера в этом не участвует.

### Кто отвечает на сообщение

Обработчиков несколько, и все умеют вести диалог, поэтому право отвечать нужно
заранее разграничить. `Router` идёт по списку и останавливается на первом, кто
вернул `handled = true`; остальные событие не видят. Без этого одно сообщение
вызывало бы несколько вопросов сразу, а анкеты заявки и договора мешали бы друг другу.

| Обработчик | Забирает |
| --- | --- |
| `user` | `/start`, нажатия кнопок роли |
| `listing` | `/list`, `/cancel`; обычный текст арендодателя с незавершённой заявкой; обычный текст арендатора, если это шестизначный код и он ещё не подключён |
| `contract` | `/data`, `/redata`, `/contract`; обычный текст, когда анкета стороны ещё не заполнена |
| `info` | `/info` |

Порядок регистрации в `internal/app/dependency.go` значим: обработчик договора
забирает обычный текст последним, поэтому адрес и цена уходят в анкету заявки.
Проверяется это тестами в `internal/app/routing_test.go`.

Webhook-режим (`Api.GetHandler`) в проекте не используется — бот работает на long polling.

