# auth

Каркас сервиса пользователей и сессий по `api/auth.yaml`.

- `GET /healthz` — `204 No Content`, если процесс отвечает; БД не проверяется.
- `GET /readyz` — `204 No Content` при успешном ping `auth_db`, иначе `503`.
  Проверка ограничена `AUTH_READINESS_TIMEOUT`; тело ответа пустое.
- Login, refresh, logout, me и JWKS пока возвращают `501 Not Implemented`
  в формате `application/problem+json`. Сессии и токены пока не выдаются.

Сервис запускается при недоступном PostgreSQL и восстанавливает readiness после
возвращения БД. Доступность таблиц и ключа JWT будет проверяться при реализации
соответствующих функций; на этом этапе readiness проверяет только соединение.
`SIGINT`/`SIGTERM` запускают graceful shutdown HTTP с таймаутом
`AUTH_SHUTDOWN_TIMEOUT`, затем закрывается пул PostgreSQL. Логи процесса — JSON.
В каждый HTTP-ответ добавляется `X-Trace-ID`; ошибки содержат тот же `trace_id`.

## Локальный запуск

Из корня репозитория добавьте блок `AUTH_*` из `.env.example` в свою `.env`.
Пароль в `AUTH_DATABASE_URL` должен совпадать с `AUTH_DB_PASSWORD`.
Для нового окружения создайте `.env` по примеру из корневого README.

Запустите PostgreSQL; init-скрипт нового тома создаёт `auth_db` и `auth_user`:

```sh
docker compose --env-file .env -f deploy/docker-compose.yml up -d postgres
make auth-db-up
```

Загрузите переменные и запустите auth:

```sh
set -a
source .env
set +a
go run ./services/auth/cmd/service
```

По умолчанию процесс слушает `:8082`. В другом терминале:

```sh
curl -i http://localhost:8082/healthz
curl -i http://localhost:8082/readyz
```

Через существующий gateway доступны `/internal/auth/healthz` и
`/internal/auth/readyz` на `http://localhost:8000`.
Compose-контейнер auth относится к следующим шагам реализации.

## Миграции

Схема `auth_db` содержит `organizations`, `users`, `refresh_tokens`.
Описание ограничений и команд отката — в [README миграций](migrations/README.md).
Статус миграций показывает `make auth-db-status`.

Миграция создаёт пустые таблицы; учётные записи пока не загружаются.

## Конфигурация

`AUTH_DATABASE_URL` обязателен. Остальные настройки имеют значения по умолчанию,
перечисленные в `.env.example`: HTTP-таймауты, параметры пула PostgreSQL,
readiness и shutdown. Конфигурация проверяет адрес сервера, PostgreSQL URL,
положительные таймауты и соотношение размеров пула.

Настройки JWT уже описаны: issuer `smart-credit-auth`, audience `smart-credit`,
access TTL `15m`, `kid` и путь к приватному ключу. Ключ пока не читается и не
генерируется. Его загрузка, RS256 и JWKS будут добавлены на шаге выпуска JWT.

Будущая refresh-cookie: имя `smart_credit_refresh`, путь `/api/auth`,
`HttpOnly`, `SameSite=Strict`. Для прямого обращения к auth задайте
`AUTH_REFRESH_COOKIE_PATH=/auth`, для HTTPS — `AUTH_REFRESH_COOKIE_SECURE=true`.
Имя и путь проходят проверку синтаксиса; префиксы `__Secure-` и `__Host-`
требуют соответствующих настроек безопасности.

## Проверки

```sh
go test ./services/auth/...
```

SQL-проверки запускаются отдельно:

```sh
make auth-db-test
```

Они проверяют ограничения ролей и организаций, email без учёта регистра,
refresh-хеши и сроки, а также откат миграции. Для каждого теста
создаётся отдельная схема; существующие таблицы auth не затрагиваются.
