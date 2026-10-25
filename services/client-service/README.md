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
- `GET /client/applications/{application_id}/timeline` — возвращает публичную историю заявки своей организации от старой записи к новой; для чужой или несуществующей заявки отвечает `404`.
- `PATCH /client/applications/{application_id}` — автосохраняет форму черновика по `If-Match`.
- `POST /client/applications/{application_id}/documents` — принимает демо-файл до 5 MiB и сохраняет его метаданные с `Idempotency-Key`.
- `POST /client/applications/{application_id}/submit` — проверяет заполненность черновика, переводит его в `SUBMITTED` и сохраняет событие `ApplicationSubmitted` в outbox.
- `POST /client/applications/{application_id}/info` — проверяет актуальный запрос банка, сохраняет уточнённый снимок заявки и отправляет `InfoProvided` через outbox.
- `POST /client/applications/{application_id}/offer/accept` — подтверждает выбранную актуальную оферту, переводит действие в `ACCEPT_PENDING` и отправляет `OfferAccepted` через outbox; после подтверждения банка заявка переходит к оформлению.
- `POST /client/applications/{application_id}/offer/decline` — отказывается от выбранной актуальной оферты, переводит действие в `DECLINE_PENDING` и отправляет `OfferDeclined` через outbox; окончательное закрытие заявки выполняется после подтверждения банка.
- `POST /client/applications/{application_id}/sign` — проверяет подтверждённую банком оферту и состояние `FORMALIZING`, переводит действие в `SIGN_PENDING` и отправляет `ContractSigned` через outbox.
- `POST /client/applications/{application_id}/cancel` — отзывает отправленную заявку до подписи, устанавливает `CANCEL_PENDING` и отправляет `ApplicationCancelled` через outbox. Требует `Idempotency-Key`; необязательная причина содержит до 500 символов. Черновики, завершённые заявки, подписанные договоры и заявки с pending-действием возвращают `409`. Окончательное закрытие подтверждает банк.

Технические ручки доступны без JWT и возвращают пустое тело. Бизнес-ручки
требуют Bearer JWT с ролью `CLIENT`; профиль всегда выбирается по `org_id` из
токена. При недоступной БД
сервис продолжает работать, readiness восстанавливается после её подключения.
Kafka используется для outbox relay в `client.events` и проекции входящих
`bank.events`; недоступность брокера не останавливает HTTP-сервис и не входит в
проверку readiness. Непубликованные события остаются в outbox и будут повторены
после восстановления брокера.
Демо-файлы сохраняются в локальный Docker volume `client_documents`.

В `FORMALIZING` карточка возвращает последнюю оферту в поле `offer`, включая
её ID и условия, даже если банк больше не передаёт `active_offer`. Это позволяет
вызвать `/sign` после обновления страницы без локального кеша браузера.

Загрузка принимает PDF, PNG, JPG/JPEG, DOC/DOCX, XLS/XLSX, CSV и TXT: проверяет
расширение и признаки формата в содержимом (сигнатуру PDF и маркер конца файла,
заголовок изображения, тип OLE-документа, структуру ZIP-контейнера Office).
TXT/CSV должны быть непустым текстом UTF-8 без управляющих символов, кроме
перевода строки и табуляции. CSV проверяется как таблица минимум из двух колонок
с одинаковым числом полей; разделитель — запятая, точка с запятой или табуляция.
Неподдерживаемый или нераспознанный формат возвращает `422 INVALID_DOCUMENT`
с пояснением. Это базовая проверка формата, а не полный разбор или проверка
подлинности выписки/бизнес-плана. Оригинальные байты сохраняются без изменений
в volume; размер, SHA-256 и остальные метаданные — в `application_documents`.

Загрузка сохраняет непустой файл до 5 MiB и метаданные; извлечение значений
из PDF/Excel/изображения в этом контракте отсутствует. Суммы и реквизиты клиент
заполняет в форме вручную. Пустой файл отклоняется с `422 INVALID_DOCUMENT`.
Submit с незаполненной формой или без обязательных документов возвращает
`422 INVALID_APPLICATION`: `detail` перечисляет недостающие данные. Заявка
остаётся в `DRAFT`, событие в банк не создаётся. Неполный ответ на запрос банка
даёт `422 INVALID_INFO_RESPONSE` с пояснением и сохраняет `NEEDS_INFO`.
При ответе без `document_ids` отправляется весь сохранённый набор документов.

## Запуск в Docker

Из корня репозитория добавьте переменные `AUTH_*` и `CLIENT_*` из `.env.example`
в локальный `.env`, затем выполните:

```sh
make client-up
```

Команда создаёт постоянный ключ auth, запускает Postgres и auth, применяет
миграции `auth_db`/`client_db`, собирает и запускает `client-service` с Kafka.
Используется общий Dockerfile
`deploy/docker/go-service.Dockerfile` с `SERVICE_PATH=services/client-service`.
Внутри контейнера сервис слушает `:8080` и подключается к `postgres:5432`;
на компьютере доступен по адресу `http://localhost:8083`.
Docker healthcheck вызывает `/readyz`. Клиент проверяет JWT через JWKS по
`http://auth:8080/.well-known/jwks.json`; issuer/audience берутся из `AUTH_*`,
таймаут и кеш — из `CLIENT_JWKS_TIMEOUT` и `CLIENT_JWKS_CACHE_TTL`.
В `.env` локальный `CLIENT_JWKS_URL` указывает на порт `8082`; Compose заменяет
его адресом Docker-сервиса. Токены с ролями ANALYST/ADMIN дают `403`.

```sh
make client-logs
make client-down
```

`client-down` удаляет только контейнер клиента. Auth, Postgres и данные сохраняются.
Проверить ручки через уже запущенный gateway можно по адресам
`http://localhost:8000/internal/client/healthz` и
`http://localhost:8000/internal/client/readyz` (при стандартном `GATEWAY_PORT`).
Gateway направляет `/api/client/*` в `client-service:8080`, `/api/auth/*` —
в `auth:8080`. Для Go-процессов на компьютере используйте прямые порты.

## Проверка

Из корня репозитория:

```sh
go test ./services/client-service/...
```

Регрессионные тесты SQL запускаются при заданной `CLIENT_TEST_DATABASE_URL`:

```sh
CLIENT_TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/test_db?sslmode=disable' \
  go test -count=1 ./services/client-service/internal/repository/application
```

Используйте отдельную тестовую БД. Каждый тест создаёт собственную временную
схему, применяет миграции и удаляет схему после проверки. Без переменной
интеграционные тесты пропускаются. Проверяются снимки черновиков,
protobuf-события, идемпотентность, tenant isolation и повторные
публичные снимки банка.
