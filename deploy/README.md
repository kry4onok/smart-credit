# Локальный Compose и gateway

[`docker-compose.yml`](docker-compose.yml) описывает полный демо-стек. `make demo-up` последовательно поднимает Postgres и Kafka, применяет миграции, создаёт локальные учётки, собирает сервисы, загружает демо-данные банка и запускает gateway. `web` в эту команду не входит: для браузерного кабинета выполните `make web-up`.

## Входы

- `gateway` — nginx на `127.0.0.1:${GATEWAY_PORT:-8000}`. `/docs/` показывает Swagger UI; `/api/auth/*`, `/api/client/*`, `/api/backoffice/*` направляются в соответствующий HTTP-сервис. Конфигурация: [`nginx/nginx.conf`](nginx/nginx.conf).
- `swagger-ui` — отдельный контейнер на `127.0.0.1:${SWAGGER_PORT:-8081}`, читает `api/*.yaml` из read-only mount. `make docs-up` запускает его без полного стека.
- `web` — статика и свой nginx на `127.0.0.1:${WEB_PORT:-3000}`; для браузера проксирует API по [`nginx/web.conf`](nginx/web.conf).

Технические маршруты и возможные переопределения портов перечислены в [корневом README](../README.md) и [`.env.example`](../.env.example). Ключи и пароли храните только в локальном `.env`.

## Хранилища

Postgres содержит три отдельные БД (`auth_db`, `client_db`, `bank_db`) и роли; первичная инициализация — [`postgres/init-dbs.sh`](postgres/init-dbs.sh). Kafka — один KRaft-брокер с топиками `client.events` и `bank.events`, которые создаёт `make infra-up`. Тома Compose сохраняют Postgres, ключ auth и локальные документы клиента после `make infra-down`.

## Проверка

```bash
make infra-ps
make gateway-logs
make infra-logs
```

Для проверки nginx после изменения конфигурации — `make gateway-reload`; для остановки контейнеров — `make infra-down`. Исходники бизнес-сервисов и их самостоятельные команды описаны в README соответствующих каталогов.
