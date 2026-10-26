# Локальный Compose и gateway

[`docker-compose.yml`](docker-compose.yml) описывает полный демо-стек. `make demo-up` последовательно поднимает Postgres и Kafka, применяет миграции, создаёт локальные учётки, собирает сервисы, загружает демо-данные банка, запускает gateway со Swagger и `web` (кабинет на `http://localhost:3000`). Отдельно `web` можно перезапустить командой `make web-up`.

## Входы

- `gateway` — nginx на `127.0.0.1:${GATEWAY_PORT:-8000}`. `/docs/` показывает Swagger UI по контрактам `/api/{auth,client,bank,common,assessment}.yaml`; `/api/auth/*`, `/api/client/*`, `/api/backoffice/*` направляются в соответствующий HTTP-сервис, `/internal/{auth,client,bank}/*` — во внутренние маршруты сервисов (например, `/internal/bank/*` → `bank-service:8080` внутри сети Docker). Конфигурация: [`nginx/nginx.conf`](nginx/nginx.conf).
- `swagger-ui` — отдельный контейнер на `127.0.0.1:${SWAGGER_PORT:-8081}`, читает `api/*.yaml` из read-only mount. `make docs-up` запускает его без полного стека.
- `web` — статика и свой nginx на `127.0.0.1:${WEB_PORT:-3000}`; для браузера проксирует API по [`nginx/web.conf`](nginx/web.conf).

Порты стенда перечислены в [корневом README](../README.md), их переопределения — в [`.env.example`](../.env.example); `bank-service` (8080), `auth` (8082) и `client-service` (8083) публикуются на фиксированных портах `127.0.0.1`. Ключи и пароли храните только в локальном `.env`.

## Хранилища

Postgres содержит три отдельные БД (`auth_db`, `client_db`, `bank_db`) и роли; первичная инициализация — [`postgres/init-dbs.sh`](postgres/init-dbs.sh). Kafka — один KRaft-брокер с топиками `client.events` и `bank.events`, которые создаёт `make infra-up`. Тома Compose сохраняют Postgres, ключ auth и локальные документы клиента после `make infra-down`.

## Проверка

```bash
make infra-ps
make gateway-logs
make infra-logs
```

Для проверки nginx после изменения конфигурации — `make gateway-reload`; для остановки контейнеров с сохранением данных — `make infra-down`; `make demo-reset` останавливает стек и удаляет его тома (БД, ключ auth, документы). Исходники бизнес-сервисов и их самостоятельные команды описаны в README соответствующих каталогов.
