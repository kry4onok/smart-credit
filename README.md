# smart-credit

Полное техническое задание: [TZ.md](TZ.md).

## Оценка v2 и Help

[Формулы, факторы и источники](docs/ASSESSMENT_V2.md),
[запуск и проверки](docs/ASSESSMENT_RUNBOOK.md),
[Assessment and Help API](api/assessment.yaml).

## Веб-интерфейс

Кабинет клиента и рабочее место аналитика — в [web/](web/README.md). Быстрый просмотр
без backend: `make web-install web-mock` → `http://localhost:5173` (демо-учётки и
пароль `demo` показаны на экране входа). Docker-сборка с nginx: `make web-up` →
`http://localhost:3000`.

## Проверка Go-кода

Перед pull request запустите:

```bash
make lint
```

Команда использует закреплённую версию `golangci-lint`; отдельная установка не нужна.

Сквозные сценарии для Postman и инструкция по запуску: [postman/README.md](postman/README.md).

## Swagger UI

Для проверки всех API и банковского сценария одной командой запустите
инфраструктуру, Auth, Client, Bank, scoring, bureau-mocks, assistant, демо-сиды
и gateway:

```sh
make demo-up
```

После запуска откройте `http://localhost:8000/docs/`. Повторный `demo-up`
безопасен для существующих демоданных. Frontend запускается отдельно через
`make web-up`; остановить весь Compose-стек можно через `make infra-down`.

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
`http://localhost:8000/docs/`. Auth и client запускаются в Compose;
bank сохраняет существующий адрес на компьютере:

| Маршрут gateway | Сервис |
|---|---|
| `/api/auth/*`, `/api/.well-known/*` | `auth:8080` внутри Docker-сети |
| `/api/client/*` | `client-service:8080` внутри Docker-сети |
| `/api/backoffice/*` | bank-service на `127.0.0.1:8080` |

Технические ручки получают отдельный префикс, чтобы Swagger мог определить
конкретный сервис: `/internal/auth/healthz`, `/internal/client/healthz` и
`/internal/bank/healthz`. Для технических ручек gateway удаляет этот префикс
и передаёт сервису обычный `/healthz` или `/readyz`.

Оригинал вложения отправленной заявки доступен сотрудникам банка с JWT роли
`ANALYST`/`ADMIN` по `GET /internal/client/applications/{application_id}/documents/{document_id}/content`.
Для этого маршрута gateway сохраняет полный путь до client-service.

Gateway удаляет только внешний префикс `/api`. Например,
`/api/backoffice/applications` передаётся bank-service как
`/backoffice/applications`. Если соответствующий сервис не запущен, маршрут
вернёт `502 Bad Gateway`. Gateway может запускаться до auth/client; Docker DNS
обновляется после их запуска и пересоздания. Технические ручки также доступны напрямую
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

| Подключение | Адрес с компьютера | Адрес из Compose-сервиса |
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

### Демоданные bank-service

После `make infra-up` и `make bank-db-up` загрузите отдельный seed:

```sh
make scoring-up
make bureau-up
make bank-up
make bank-demo-seed
make gateway-up
```

Seed находится в `services/bank-service/seeds/demo.sql` и не входит в миграции.
Он добавляет два продукта и семь заявок с фиксированными UUID. Повторный
запуск не дублирует записи; срок доступной демооферты продлевается на 30 дней.
Используйте его только в локальной демонстрационной БД.

| Последние цифры UUID заявки | Статус | Что видно в detail |
|---|---|---|
| `...0001` | `MANUAL_REVIEW` | Неназначенная заявка, проверки и score `B`. |
| `...0002` | `MANUAL_REVIEW` | Назначена аналитику `aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa`, score `C`. |
| `...0003` | `NEEDS_INFO` | Ошибка БКИ и открытый запрос выписки. |
| `...0004` | `COUNTER_OFFER` | Активная оферта другого продукта на меньшую сумму и срок. |
| `...0005` | `SCORING → DECISION` | Worker автоматически сохраняет реальный score; банковские правила ещё не запущены. |
| `...0006` | `ENRICHMENT → SCORING → DECISION` | Банк получает ответы bureau-mocks, сохраняет их и запускает scoring. |
| `...0007` | `ENRICHMENT → MANUAL_REVIEW` | Демо-таймаут кредитной истории; код ошибки виден в `checks.bureau_error`. |

Swagger UI: `http://localhost:8000/docs/` → **Bank API**. Проверьте
`GET /backoffice/applications` и
`GET /backoffice/applications/10000000-0000-4000-8000-000000000004`.
В detail должны быть `offer`, `score` и заголовок `ETag: "1"`; для заявки
`...0003` — `info_request` и `checks.bureau_error`.
Bank API берёт ID сотрудника из access JWT. Для назначенной заявки
`allowed_actions` рассчитывается по роли и ID вошедшего аналитика.

### Каталог банка в клиентском API

После `make bank-up` и `make bank-demo-seed` bank-service публикует полный
список активных продуктов через outbox в `bank.events`. Фоновая сверка запускается
сразу и затем каждые пять секунд. Новая версия агрегата `catalog` появляется
только при изменении публичного снимка; удаление продукта из активного каталога
тоже передаётся полным снимком. Client-service применяет его к своей проекции.

Для проверки через Swagger дополнительно запустите `make client-up`,
`make auth-dev-seed` и `make gateway-up`. На `http://localhost:8000/docs/`
войдите через Auth API `POST /auth/login` как `client@smart-credit.test`,
затем вставьте `access_token` в **Authorize** Client API и вызовите
`GET /client/products`. Ответ должен содержать `catalog_version >= 1` и
активные демопродукты `FLEX` и `WORKING`. Повторный запуск банка без изменений
не увеличивает версию. `make bank-catalog-test` проверяет это на PostgreSQL.

## Как добавить HTTP Go-сервис в Docker

Для Go-сервисов используется общий `deploy/docker/go-service.Dockerfile`.
Копировать Dockerfile в каталог своего сервиса не нужно. Чтобы подключить новый
сервис:

1. Добавьте запускаемый пакет `<service-path>/cmd/service`.
2. Настройте чтение адреса HTTP-сервера и зависимостей из env.
3. Добавьте сервис в `deploy/docker-compose.yml` по шаблону ниже.
4. Добавьте команды `<service>-up`, `<service>-down` и `<service>-logs` в
   `Makefile`.
5. Проверьте прямые `/healthz`, `/readyz` и технический маршрут через gateway.

Все HTTP Go-сервисы внутри контейнера слушают порт `8080`. Разные порты используются
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

Связку auth/client поднимает `make client-up`, затем `make gateway-up`.
Эти команды не запускают банк и его зависимости. Для отдельного окружения
можно передать `make ENV_FILE=/path/to/test.env ...`; по умолчанию используется `.env`.

## auth

Auth запускается из `services/auth/cmd/service` и по умолчанию слушает
порт `8082`. Реализованы `/healthz`, `/readyz` с проверкой `auth_db` и публичный
`/.well-known/jwks.json`, а также login/refresh/logout с Argon2id, RS256 JWT
и refresh-cookie на семь дней. Refresh не ротируется, logout его отзывает.
`GET /auth/me` проверяет Bearer JWT и возвращает актуальный профиль и организацию
из БД. Общий HTTP-слой добавляет trace ID, безопасные JSON-логи и обработку panic.
`make auth-up` собирает auth, создаёт ключ в volume `auth_keys`, применяет миграции
и запускает сервис. Существующий ключ сохраняется; контейнер работает от `app`.
Ключ и refresh-сессии переживают пересоздание контейнера.
Для запуска Go-процесса на компьютере создайте отдельный локальный ключ командой
`go run ./services/auth/cmd/keygen`.
Настройки `AUTH_*` находятся
в `.env.example`, инструкции запуска — в [README auth](services/auth/README.md).

После запуска PostgreSQL примените `make auth-db-up`. Миграция создаёт пустые
таблицы `organizations`, `users`, `refresh_tokens`.
Проверка миграций в изолированных схемах: `make auth-db-test`.
Для локального входа задайте три `AUTH_DEV_*_PASSWORD` в игнорируемом Git `.env`
и выполните `make auth-dev-seed`. Команда создаёт ANALYST, ADMIN и CLIENT с
фиксированными UUID; детали и email приведены в [README auth](services/auth/README.md).
Интеграционная проверка seed в отдельной схеме: `make auth-dev-seed-test`.

## client-service

Сервис кабинета клиента можно собрать и запустить в Docker вместе с Postgres:

```sh
make client-up
```

Команда запускает auth с постоянным ключом, поднимает Postgres и Kafka,
применяет миграции обеих БД, собирает общий Go-образ клиента и ждёт `/readyz`.
Клиент получает JWKS по `http://auth:8080/.well-known/jwks.json`.
Логи и остановка контейнера:

```sh
make client-logs
make client-down
```

Для отдельного применения миграций используются команды:

```sh
make client-db-up
make client-db-status
```

Проверка процесса и подключения к `client_db`:

```sh
curl -i http://localhost:8083/healthz
curl -i http://localhost:8083/readyz
```

Обе ручки отвечают `204 No Content`, когда сервис и Postgres готовы.
Бизнес-ручки требуют Bearer JWT с ролью `CLIENT`; организация выбирается по
`org_id` из проверенного токена. Для Go-процесса на компьютере JWKS берётся с
`http://127.0.0.1:8082/.well-known/jwks.json`, в Compose — с
`http://auth:8080/.well-known/jwks.json`.


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

Обе ручки отвечают `204 No Content`, когда сервис и Postgres готовы.
Реализованы список, карточка, взятие дела в работу (`take`), решение, аудит
и резюме. Для demo-аналитика используется
`BANK_DEMO_ANALYST_ID` из `.env.example`.

Банковский scoring adapter в `services/bank-service/internal/client/scoring`
принимает типизированный снимок компании и успешные ответы БКИ,
сохраняет различие между отсутствующим значением и известным нулём, вызывает
gRPC с общим дедлайном 2 с и проверяет ответ модели. Попытка хранит
`scoring_input_revision`, дату и исходный запрос; карточка читает только
завершённую текущую попытку. Для подготовленного дела с двумя последними
успешными ответами БКИ этап можно запустить явно:

```sh
make scoring-up
make bank-up
make bank-demo-seed
make gateway-up
make bank-scoring-run CASE_ID=10000000-0000-4000-8000-000000000001
```

После успешной оценки карточка этого дела в Swagger показывает
`model_version: monotone-scorecard-v1`. Повторный запуск дела с завершённой
оценкой запрещён до смены `scoring_input_revision`; техническую ошибку можно
повторить. При запуске bank-service из Compose scoring worker автоматически
подбирает дела в статусе `SCORING` (например, `...0005`) каждые 2 секунды.
После оценки `ASSESSED` дело переходит в `DECISION`; недостаток данных или
техническая ошибка ведут в `MANUAL_REVIEW` с записью в аудите. Worker использует
короткую аренду в `bank_db` и после перезапуска завершает зависшую попытку как
техническую ошибку. Для локального `make bank-run` задайте
`BANK_SCORING_GRPC_ADDR=127.0.0.1:50051` в `.env`; без адреса worker выключен.
Worker enrichment автоматически выбирает дела `ENRICHMENT`, вызывает два метода
`bureau-mocks` с дедлайном 2 секунды на метод и одним повтором `UNAVAILABLE`.
Сырые успешные ответы или коды ошибок сохраняются в `enrichment_results`
вместе с переходом и аудитом. Успех ведёт в `SCORING`, откуда дело подхватывает
scoring worker; ошибка источника ведёт в `MANUAL_REVIEW`. Вызовы gRPC идут вне
SQL-транзакции. Для локального `make bank-run` нужен адрес
`BANK_BUREAU_GRPC_ADDR=127.0.0.1:50053` в `.env`.
Решение по правилам банка и Kafka consumer пока не подключены.

## Bureau mocks

`bureau-mocks` — отдельный Go gRPC-сервис без БД и внешнего БКИ. Он читает
вымышленные профили из `seed/personas.yaml` при запуске и по ИНН отвечает на
`GetCompanyProfile` (статус и дата регистрации компании) и `GetCreditHistory`
(долг и просрочки). Ответы для одного ИНН повторяемы. Два метода независимы:
ошибка кредитной истории не мешает получить профиль компании.

```sh
make bureau-up
make bureau-smoke
make bureau-logs
make bureau-down
```

Порт на компьютере — `127.0.0.1:50053` (переменная `BUREAU_PORT`), внутри
Compose — `bureau-mocks:50053`. `grpc.health.v1.Health/Check` сообщает
`SERVING` для `smartcredit.bureau.BureauService`; reflection включается только
при `BUREAU_ENABLE_REFLECTION=true`. Валидный ИНН, которого нет в seed,
возвращает `NOT_FOUND`; неверная контрольная сумма — `INVALID_ARGUMENT`.
Сценарий `5401010012` задерживает кредитную историю на 2,5 секунды для
проверки дедлайна банка; `1234567894` возвращает `UNAVAILABLE`.
`1111111117` показывает ликвидацию компании как стоп-фактор.
Известный нулевой долг в `9876543210` остаётся нулём, а отсутствующие поля у
`500100732259` остаются отсутствующими.

В банковском demo seed ИНН заявок `...0003` и `...0004` исправлены на валидные
`5401010012` и `6671160003`. Если эти заявки уже были загружены в локальную
БД до обновления, повторный `make bank-demo-seed` их не перезапишет. Новые
демо-дела `...0006` и `...0007` используют актуальные ИНН; их можно добавить
повторным запуском seed без удаления существующей БД:

```sh
make scoring-up
make bureau-up
make bank-up
make bank-demo-seed
```

После запуска банка `...0006` должен дойти до `DECISION`, а `...0007` — до
`MANUAL_REVIEW` с `BUREAU_TIMEOUT`. Эти переходы ещё не являются банковским
решением об одобрении или отказе.

## Scoring v1

Готов отдельный Python gRPC-сервис: риск бизнеса 0..1000, grade A–E,
четыре вклада факторов, до трёх причин и явные результаты для недостаточных
или неприменимых данных. Это экспертная монотонная модель, не оценка PD.
Банк принимает кредитное решение и проверяет продукт, документы и стоп-факторы.

- [Запуск scoring и подключение backend](ml/scoring/README.md).
- [Общие профили для bureau mocks и интеграции](seed/scoring/personas.json).

Нужны Docker, Go из `go.mod` и uv (проверено с 0.12.3). Scoring запускается
из общего `deploy/docker-compose.yml`. Для первого запуска подготовьте `.env`
по разделу «Локальная инфраструктура». `scoring-up` запускает только scoring,
без Postgres/Kafka:

```sh
make scoring-sync
make scoring-up
make scoring-smoke
make scoring-smoke-go
make bank-scoring-smoke
```

Host endpoint — `127.0.0.1:50051`; стандартный gRPC health. Внутри общей
Compose-сети backend использует `scoring:50051`. Все сервисы описаны в общем
`deploy/docker-compose.yml` и используют одну сеть.
`bank-scoring-smoke` вызывает именно банковский adapter: он собирает запросы из
типизированных данных, отправляет их по gRPC и проверяет ответы для обычного
score, известного нулевого долга, score 0 и отсутствующего долга. Успех —
четыре `PASS` в выводе теста. Проверка не меняет `bank_db` и карточку дела:
adapter пока не подключён к worker.
Для нестандартного порта: `make scoring-up SCORING_PORT=50052`, затем
`make scoring-smoke scoring-smoke-go bank-scoring-smoke SCORING_TARGET=127.0.0.1:50052`.

```sh
make scoring-test scoring-lint
make gen-proto-check
make scoring-evaluate
make scoring-down
```

`scoring-evaluate` сохраняет клиентские измерения в
`/tmp/smart-credit-scoring-evaluation.json`; они не оценивают точность дефолта.
Для запуска без Docker: `make scoring-run` (по умолчанию loopback; переопределение
через `SCORING_ADDR`). `SCORING_MODEL_PATH` позволяет явно выбрать артефакт;
без него используется включённый в пакет конфиг. Python 3.12.12 и зависимости
закреплены в `ml/scoring`; generated Python imports требуют
`PYTHONPATH=proto/gen/python`, как уже задано в Makefile и контейнере.

Все пять proto генерируются командой `make gen-proto`; `make gen` теперь
включает OpenAPI и protobuf. Generated файлы не редактировать вручную.
CI `.github/workflows/scoring.yml` проверяет воспроизводимость генерации,
модель, gRPC, Go и оба клиента против контейнера.

Вызов scoring из банковского конвейера, сохранение текущей revision/attempt и
отображение результата в карточке подключает backend по инструкции scoring. Наличие
этого сервиса не означает готовность всего банковского E2E.

## ML assistant (ML-2)

Готов gRPC-сервис с двумя методами из `proto/assistant/assistant.proto`:
`SummarizeCase` даёт аналитику короткое резюме дела, `ExplainDecision` формирует
текст по уже принятому решению для клиента. Решение, score и параметры оферты
assistant не меняет. Сервис запускается в общем Docker Compose; локальная Ollama
с моделью `qwen2.5:3b` работает на компьютере вне Docker.

Для локального запуска из корня репозитория установите и запустите Ollama,
затем выполните:

```sh
ollama pull qwen2.5:3b
cp .env.example .env  # только если .env ещё нет
make assistant-up
make assistant-smoke
```

`assistant-smoke` прогоняет семь банковских demo-сценариев: семь резюме и два
объяснения по делам №3 и №4. Скрипт требует ответа именно выбранной модели;
при успехе в строках стоит её имя, например `[qwen2.5:3b]`. На компьютере сервис доступен по
`127.0.0.1:50052`, внутри Compose — по `assistant:50052`. Контейнер обращается
к Ollama через `host.docker.internal:11434`. Порт и модель задаются через
`ASSISTANT_PORT`, `OLLAMA_MODEL`, `OLLAMA_URL` в `.env`. Эксперты могут задать
свой `LLM_PROVIDER=ollama|openai`, `LLM_BASE_URL`, `LLM_MODEL` и, если нужен,
`LLM_API_KEY`. Пустые новые переменные используют прежние настройки Ollama.
Логи и остановка:
`make assistant-logs`, `make assistant-down`.

При недоступной Ollama или непроверенном ответе модели assistant возвращает
безопасный шаблон с `model_version=template`. Для клиентского текста он
использует только публичную причину и не получает внутренний комментарий
аналитика. Подробная проверка fallback и запуск без Docker описаны в
[инструкции ML-2](ml/assistant/README.md).

**Стык с bank-service:** `POST /backoffice/applications/{id}/summary` вызывает
`SummarizeCase` с дедлайном 5 секунд по умолчанию. Для медленной локальной модели
можно задать `BANK_ASSISTANT_TIMEOUT=20s` (меньше `BANK_HTTP_WRITE_TIMEOUT`) и
пересоздать bank-service. Compose передаёт адрес `assistant:50052`
через `BANK_ASSISTANT_GRPC_ADDR`; при недоступности gRPC bank-service возвращает
собственный шаблон с `source=template`. `model_version=template` от assistant
тоже переводится в `source=template`. Для резюме банк передаёт текущий статус,
человекочитаемую цель, описания причин из результата скоринга и описания
известных банковских правил. Неизвестный код остаётся кодом без догадки о смысле.
При ручном решении банк передаёт в `ExplainDecision` проверенную оферту до транзакции, сохраняет клиентский текст и публикует его в существующем событии; при ошибке ассистента использует исходную публичную причину или шаблон для одобрения.
Поля запросов и ответственность сторон
описаны в [инструкции ML-2](ml/assistant/README.md#контракт-с-bank-service).

**Проверка:** unit-тесты и контейнерный fallback проверяет
[CI assistant](.github/workflows/assistant.yml) без загрузки весов. Ответы
реальной модели, качество текста и задержку при банковском дедлайне
проверьте локально через `make assistant-smoke`; сценарии и чеклист описаны в
[инструкции ML-2](ml/assistant/README.md#проверка-качества-текста).

Для общего демо ориентир — компьютер с **16 ГБ ОЗУ**, из которых Docker можно
выделить примерно 5–6 ГБ; Ollama работает в оставшейся памяти компьютера.
Это оценка: полный стек ещё не замерен при одновременном запуске. Сведения
о модели и её лицензии находятся в
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
