# smart-credit

## Проверка Go-кода

Перед pull request запустите:

```bash
make lint
```

Команда использует закреплённую версию `golangci-lint`; отдельная установка не нужна.

## Swagger UI

Общая документация Auth, Client и Bank API запускается отдельно от сервисов:

```sh
make docs-up
```

Swagger UI доступен по адресу `http://localhost:8081`. Нужный API выбирается в
верхней панели. Файлы из `api/` подключены к контейнеру только для чтения,
поэтому изменения контрактов видны после обновления страницы без пересборки
образа. Порт задаётся через `SWAGGER_PORT` в `.env`.

```sh
make docs-logs
make docs-down
```

Для вызова API из Swagger запустите общий dev gateway:

```sh
make gateway-up
```

Swagger с рабочим `Try it out` доступен по адресу
`http://localhost:8000/docs/`. Gateway ожидает локальные сервисы на следующих
портах:

| Маршрут gateway | Локальный сервис |
|---|---|
| `/api/auth/*`, `/api/.well-known/*` | auth-service на `127.0.0.1:8082` |
| `/api/client/*` | client-service на `127.0.0.1:8083` |
| `/api/backoffice/*` | bank-service на `127.0.0.1:8080` |

Технические ручки получают отдельный префикс, чтобы Swagger мог определить
конкретный сервис: `/internal/auth/healthz`, `/internal/client/healthz` и
`/internal/bank/healthz`. Gateway удаляет этот префикс и передаёт сервису
обычный `/healthz` или `/readyz`.

Gateway удаляет только внешний префикс `/api`. Например,
`/api/backoffice/applications` передаётся bank-service как
`/backoffice/applications`. Если соответствующий сервис не запущен, маршрут
вернёт `502 Bad Gateway`. Технические ручки также остаются доступны напрямую
на порту конкретного сервиса.

```sh
make gateway-logs
make gateway-down
```

## Локальная инфраструктура

Из корня репозитория:

```sh
cp .env.example .env
make infra-up
make bank-db-up
```

`infra-up` запускает Postgres и Kafka и создаёт топики `client.events` и
`bank.events`. Postgres при первом запуске создаёт три независимые базы и роли:
`auth_db` / `auth_user`, `client_db` / `client_user`, `bank_db` / `bank_user`.
Пароли и порты задаются в локальном `.env`; примерные пароли предназначены
только для разработки.

| Подключение | Адрес с компьютера | Адрес из будущего Compose-сервиса |
|---|---|---|
| Postgres | `127.0.0.1:5432` | `postgres:5432` |
| Kafka | `localhost:9092` | `kafka:19092` |

Если порты заняты, измените `POSTGRES_PORT` и `KAFKA_PORT` в `.env`. Для
локального Go-сервиса используйте соответствующий пароль из `.env` и адрес
`127.0.0.1:${POSTGRES_PORT}`. Миграцию банка применяет `make bank-db-up`;
состояние показывает `make bank-db-status`.

`make infra-ps` показывает контейнеры, `make infra-logs` — их логи,
`make infra-down` останавливает контейнеры. Данные Postgres сохраняются в томе,
а локальные данные Kafka создаются заново при следующем запуске. Скрипт
создания БД выполняется только при создании пустого тома Postgres: изменение
паролей в `.env` после первого запуска само по себе не меняет пароли
существующих ролей.

## Как добавить Go-сервис в Docker

Для Go-сервисов используется общий `deploy/docker/go-service.Dockerfile`.
Копировать Dockerfile в каталог своего сервиса не нужно. Чтобы подключить новый
сервис:

1. Добавьте запускаемый пакет `<service-path>/cmd/service`.
2. Настройте чтение адреса HTTP-сервера и зависимостей из env.
3. Добавьте сервис в `deploy/docker-compose.yml` по шаблону ниже.
4. Добавьте команды `<service>-up`, `<service>-down` и `<service>-logs` в
   `Makefile`.
5. Проверьте прямые `/healthz`, `/readyz` и технический маршрут через gateway.

Все Go-сервисы внутри контейнера слушают порт `8080`. Разные порты используются
только на компьютере: bank — `8080`, auth — `8082`, client — `8083`.

```yaml
client-service:
  build:
    context: ..
    dockerfile: deploy/docker/go-service.Dockerfile
    args:
      SERVICE_PATH: services/client-service
  environment:
    CLIENT_HTTP_ADDR: ":8080"
    CLIENT_DATABASE_URL: postgres://client_user:${CLIENT_DB_PASSWORD}@postgres:5432/client_db?sslmode=disable
  depends_on:
    postgres:
      condition: service_healthy
  ports:
    - "127.0.0.1:8083:8080"
  healthcheck:
    test: ["CMD-SHELL", "wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1"]
    interval: 5s
    timeout: 3s
    retries: 20
  restart: unless-stopped
```

Из контейнеров Postgres доступен как `postgres:5432`, Kafka — как
`kafka:19092`. В `.env` нельзя добавлять секреты для общего окружения: в Git
попадают только названия переменных и безопасные локальные значения из
`.env.example`.

Пока каждый сервис запускается своей командой. После добавления запускаемых
auth и client контейнеров появятся общие команды `make dev-up`,
`make dev-down` и `make dev-logs`, которые поднимут инфраструктуру, применят
миграции и запустят весь стенд.

## bank-service

Сервис можно собрать и запустить в Docker вместе с Postgres:

```sh
make bank-up
```

Команда поднимает Postgres, применяет миграции `bank_db`, собирает общий
Go-образ с параметром `SERVICE_PATH=services/bank-service` и ждёт успешного
`/healthz`. Логи и остановка контейнера:

```sh
make bank-logs
make bank-down
```

Контейнер подключается к Postgres по внутреннему адресу `postgres:5432`, а
порт `8080` публикуется только на `127.0.0.1`. Поэтому существующий gateway
может обращаться к банку так же, как при локальном запуске.

Для запуска сервиса непосредственно на компьютере сначала поднимите
инфраструктуру и примените миграции:

```sh
make infra-up
make bank-db-up
make bank-run
```

Проверка процесса и подключения к `bank_db`:

```sh
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```

Обе ручки отвечают `204 No Content`, когда сервис и Postgres готовы. Остальные
маршруты bank API до реализации возвращают `501 Not Implemented`.
