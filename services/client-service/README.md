# client-service

HTTP-сервис кабинета клиента с техническими ручками и чтением профиля компании:

- `GET /healthz` — `204 No Content`, если процесс отвечает; БД не проверяется.
- `GET /readyz` — `204 No Content` при успешном ping `client_db`, иначе `503 Service Unavailable`. Время проверки ограничено `CLIENT_READINESS_TIMEOUT`.
- `GET /client/company` — возвращает профиль компании текущей организации из `org_id` проверенного JWT. При отсутствии профиля возвращает `404` в формате `application/problem+json`.
- `PUT /client/company` — создаёт или обновляет профиль. Для создания ETag не нужен; для обновления используется `If-Match: "<version>"`.
- `GET /client/products` — возвращает клиентскую копию каталога банка; параметр `purpose` фильтрует продукты по назначению.
- `GET /client/products/{product_id}` — возвращает условия и обязательные документы выбранного продукта.
- `POST /client/applications` — создаёт черновик заявки со снимками выбранного продукта и профиля компании; требует `Idempotency-Key`.
- `GET /client/applications` — возвращает список заявок организации с фильтром по статусу и cursor-пагинацией.
- `GET /client/applications/{application_id}` — возвращает карточку заявки, форму, документы, активный запрос банка или оферту и вычисленные действия.
- `PATCH /client/applications/{application_id}` — автосохраняет форму черновика по `If-Match`.
- `POST /client/applications/{application_id}/documents` — принимает демо-файл до 5 MiB и сохраняет его метаданные с `Idempotency-Key`.
- `POST /client/applications/{application_id}/submit` — проверяет заполненность черновика, переводит его в `SUBMITTED` и сохраняет событие `ApplicationSubmitted` в outbox.

Технические ручки доступны без JWT и возвращают пустое тело. Бизнес-ручки
требуют Bearer JWT с ролью `CLIENT`; профиль всегда выбирается по `org_id` из
токена. При недоступной БД
сервис продолжает работать, readiness восстанавливается после её подключения.
Остальные операции контракта пока возвращают `501 Not Implemented`.
Kafka используется для outbox relay в `client.events` и проекции входящих
`bank.events`; недоступность брокера не останавливает HTTP-сервис и не входит в
проверку readiness. Непубликованные события остаются в outbox и будут повторены
после восстановления брокера.
Демо-файлы сохраняются в локальный Docker volume `client_documents`.

## Запуск в Docker

Из корня репозитория добавьте переменные `CLIENT_*` из `.env.example`
в локальный `.env`, затем выполните:

```sh
make client-up
```

Команда запускает Postgres, применяет миграции `client_db`, собирает и
запускает только `client-service`. Используется общий Dockerfile
`deploy/docker/go-service.Dockerfile` с `SERVICE_PATH=services/client-service`.
Внутри контейнера сервис слушает `:8080` и подключается к `postgres:5432`;
на компьютере доступен по адресу `http://localhost:8083`.
Docker healthcheck вызывает `/healthz`; готовность БД проверяется через `/readyz`.

```sh
make client-logs
make client-down
```

`client-down` удаляет только контейнер клиента. Postgres и его данные сохраняются.
Проверить ручки через уже запущенный gateway можно по адресам
`http://localhost:8000/internal/client/healthz` и
`http://localhost:8000/internal/client/readyz` (при стандартном `GATEWAY_PORT`).
