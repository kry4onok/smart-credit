# Архитектура

Документ описывает фактическую топологию из `deploy/docker-compose.yml`, `deploy/nginx/*.conf` и кода сервисов. Контракты: OpenAPI в `api/*.yaml` (`auth`, `client`, `bank`, `assessment`, `common`), gRPC и события в `proto/*/*.proto`. Схемы БД — в [DATA_MODEL.md](DATA_MODEL.md), проверки — в [TEST_SCENARIOS.md](TEST_SCENARIOS.md).

## Компоненты

```mermaid
flowchart LR
    browser["Браузер<br/>React SPA"]
    swagger_user["Postman / Swagger<br/>разработчик"]

    subgraph compose["Docker Compose: deploy/docker-compose.yml"]
        web["web<br/>nginx + статика<br/>:3000"]
        gateway["gateway<br/>nginx, dev<br/>:8000"]
        swagger["swagger-ui<br/>:8081"]
        auth["auth<br/>Go, :8082"]
        client["client-service<br/>Go, :8083"]
        bank["bank-service<br/>Go, :8080"]
        bureau["bureau-mocks<br/>Go gRPC, :50053"]
        scoring["scoring<br/>Python gRPC, :50051"]
        assistant["assistant<br/>Python gRPC, :50052"]
        kafka[("Kafka KRaft<br/>client.events<br/>bank.events")]
        subgraph pg["PostgreSQL 16"]
            auth_db[("auth_db")]
            client_db[("client_db")]
            bank_db[("bank_db")]
        end
        docs_volume[("volume<br/>client_documents")]
    end

    ollama["Ollama на хосте<br/>:11434, qwen2.5:7b"]

    browser -->|"/api/auth, /api/.well-known"| web
    browser -->|"/api/client"| web
    browser -->|"/api/backoffice"| web
    web -->|"/auth/*"| auth
    web -->|"/client/*"| client
    web -->|"/backoffice/*"| bank

    swagger_user --> gateway
    gateway -->|"/docs/"| swagger
    gateway -->|"/api/auth"| auth
    gateway -->|"/api/client"| client
    gateway -->|"/api/backoffice"| bank

    auth --- auth_db
    client --- client_db
    client --- docs_volume
    bank --- bank_db

    client -->|"JWKS"| auth
    bank -->|"JWKS"| auth

    client -->|"outbox: ApplicationSubmitted,<br/>InfoProvided, OfferAccepted/Declined,<br/>ContractSigned, ApplicationCancelled"| kafka
    kafka -->|"client.events"| bank
    bank -->|"outbox: ApplicationPublicStatePublished,<br/>ProductCatalogPublished"| kafka
    kafka -->|"bank.events"| client

    bank -->|"gRPC BureauService"| bureau
    bank -->|"gRPC ScoringService"| scoring
    bank -->|"gRPC AssistantService,<br/>AssistantHelpService"| assistant
    client -->|"gRPC AssistantHelpService<br/>CLIENT_ASSISTANT_TARGET"| assistant
    bank -->|"HTTP GET /internal/client/...<br/>BANK_CLIENT_DOCUMENT_URL"| client
    assistant -->|"HTTP /api/generate"| ollama
```

| Компонент | Порт на хосте | Ответственность |
|---|---|---|
| `web` | 3000 | Единая точка входа: статика React и прокси `/api/auth`, `/api/.well-known`, `/api/client`, `/api/backoffice` внутри compose-сети. Остальные `/api/*` → 404. |
| `gateway` | 8000 | Dev-шлюз для Swagger и Postman: `/docs/` → swagger-ui, `/api/auth`, `/api/client`, `/api/backoffice`, `/internal/*` (readyz) внутри compose-сети. |
| `swagger-ui` | 8081 | Swagger UI по `api/auth.yaml`, `client.yaml`, `bank.yaml`, `assessment.yaml`. |
| `auth` | 8082 | Логин, refresh-cookie, `me`, выпуск RS256 JWT, `/.well-known/jwks.json`. Роли `CLIENT`, `ANALYST`, `ADMIN`. БД `auth_db`. |
| `client-service` | 8083 | Профиль компании, каталог (проекция), черновики, документы, отправка и действия клиента, Help клиента, проекция публичной оценки. Файлы — в volume `client_documents`. БД `client_db`. |
| `bank-service` | 8080 | Конвейер дела (validation → enrichment → scoring → assessment), очередь и решения аналитика, повторная оценка, оферты, истечение, подпись, демо-выдача, аудит, публикация каталога. БД `bank_db`. |
| `bureau-mocks` | 50053 | gRPC-моки внешних источников по `seed/personas.yaml`: профиль компании, кредитная история, синтетические источники оценки. |
| `scoring` | 50051 | Python gRPC `ScoringService/Score`: детерминированная скоркарта `monotone-scorecard-v1` (`ml/scoring/models/scorecard-v1.json`). В v2 её результат хранится как `baseline_v1`. |
| `assistant` | 50052 | Python gRPC: `AssistantService` (резюме дела, текст решения) и `AssistantHelpService/Help`. Вызывает Ollama, при сбое — детерминированный шаблон. |
| `postgres` | 5432 | PostgreSQL 16.15, три БД с отдельными ролями (`deploy/postgres/init-dbs.sh`). |
| `kafka` | 9092 (внутри 19092) | Apache Kafka 3.9.2 KRaft, топики `client.events` и `bank.events` по одной партиции (`make infra-up`). |
| Ollama | 11434 | Запускается на хосте, не входит в compose. Модель по умолчанию `OLLAMA_MODEL=qwen2.5:7b`. |

Swagger UI ссылается на `/api/assessment.yaml`; в `deploy/nginx/nginx.conf` статическая раздача спецификаций ограничена `auth|client|bank|common`, поэтому Assessment and Help API надёжно открывается напрямую на `http://localhost:8081`.

## Ключевые свойства

* **Изоляция данных.** Каждый сервис владеет своей БД. Межбазовых внешних ключей нет (`org_id`, `client_user_id`, `assignee_id` — просто UUID). client-service и bank-service не читают чужие БД: состояние передаётся только событиями Kafka.
* **Outbox и идемпотентность.** Бизнес-изменение и событие пишутся в одной транзакции в таблицу `outbox`; relay публикует сохранённый protobuf `Event` без изменений, соблюдая порядок `aggregate_version`. Получатель вставляет `event_id` в `processed_events` в той же транзакции, что и изменение, и подтверждает offset Kafka после commit. Повторы HTTP-команд защищены `idempotency_keys` (заголовок `Idempotency-Key`), конкурентные изменения — `If-Match` с версией.
* **Защита документа.** Аналитик получает содержимое файла через bank-service: bank вызывает `GET /internal/client/applications/{id}/documents/{doc}/content` у client-service с токеном сотрудника (маршрут доступен только ролям staff) и сверяет SHA-256 с метаданными дела.
* **JWT.** auth подписывает access-токены, client-service и bank-service проверяют подпись по JWKS (`CLIENT_JWKS_URL`, `BANK_JWKS_URL`) с кешем.
* **Без автоматической оферты.** Конвейер v2 заканчивается в `MANUAL_REVIEW`, `NEEDS_INFO` или `REJECTED` (`services/bank-service/internal/repository/assessment/repository.go`, `RunOnce`). Положительный исход всегда подтверждает аналитик: решение `APPROVE`/`COUNTER` принимается только с условиями, совпадающими с актуальным кандидатом сохранённой оценки (`CheckCurrentOffer`); иначе 422. Прежний код автооферты в `repository/decision` не вызывается: worker решения делегирует в `assessmentrepository.RunOnce`.
* **LLM не принимает решений.** Модель только формулирует текст резюме, пояснения и Help из фактов, переданных backend. Для Help Qwen выбирает лишь разрешённые ID утверждений и действий из каталога `pkg/assistanthelp/catalog.json`; Go заново рендерит текст. Статус дела, сумма, ставка и исход модель не меняет. При недоступности модели используется шаблон.
* **Синтетические источники.** Все внешние сведения помечены `is_mock=true`; список моков — в `THIRD_PARTY_NOTICES.md`, раздел «Смоделированные интеграции».

## Статусы дела в bank-service

Допустимые значения — `CHECK` в `services/bank-service/migrations/00001_bank_init.sql`; переходы — предикаты `WHERE status = ...` в `services/bank-service/internal/repository/*`. `SUBMITTED` есть в ограничении и как значение по умолчанию, но код создаёт дело сразу в `VALIDATION` (или `MANUAL_REVIEW`).

```mermaid
stateDiagram-v2
    [*] --> VALIDATION: ApplicationSubmitted
    [*] --> MANUAL_REVIEW: неизвестный продукт или политика, конфликт метаданных документа

    VALIDATION --> ENRICHMENT: данные полны
    VALIDATION --> NEEDS_INFO: не хватает полей или документов
    VALIDATION --> MANUAL_REVIEW: некорректный снимок

    ENRICHMENT --> SCORING: ответы bureau-mocks или BUREAU_UNAVAILABLE
    SCORING --> DECISION: baseline v1 или код ошибки

    DECISION --> MANUAL_REVIEW: оценка готова к подтверждению или NO_AFFORDABLE_OPTION
    DECISION --> NEEDS_INFO: не хватает требований или финансовых данных
    DECISION --> REJECTED: стоп-фактор допуска

    NEEDS_INFO --> VALIDATION: InfoProvided
    NEEDS_INFO --> NEEDS_INFO: InfoProvided v2, требования не закрыты
    NEEDS_INFO --> MANUAL_REVIEW: InfoProvided v2 требует проверки

    MANUAL_REVIEW --> MANUAL_REVIEW: take, назначение аналитика
    MANUAL_REVIEW --> OFFER: решение APPROVE
    MANUAL_REVIEW --> COUNTER_OFFER: решение COUNTER
    MANUAL_REVIEW --> NEEDS_INFO: решение REQUEST_INFO
    MANUAL_REVIEW --> REJECTED: решение REJECT
    MANUAL_REVIEW --> VALIDATION: recheck
    NEEDS_INFO --> VALIDATION: recheck

    OFFER --> OFFER_ACCEPTED: OfferAccepted
    COUNTER_OFFER --> OFFER_ACCEPTED: OfferAccepted
    OFFER --> CANCELLED: OfferDeclined
    COUNTER_OFFER --> CANCELLED: OfferDeclined
    OFFER --> EXPIRED: истёк expires_at
    COUNTER_OFFER --> EXPIRED: истёк expires_at

    OFFER_ACCEPTED --> SIGNING: ContractSigned
    SIGNING --> DISBURSED: демо-выдача через 3 с

    VALIDATION --> CANCELLED: ApplicationCancelled
    MANUAL_REVIEW --> CANCELLED: ApplicationCancelled
    OFFER_ACCEPTED --> CANCELLED: ApplicationCancelled

    DISBURSED --> [*]
    REJECTED --> [*]
    CANCELLED --> [*]
    EXPIRED --> [*]
```

Отзыв (`ApplicationCancelled`) принимается из любого статуса от `SUBMITTED` до `OFFER_ACCEPTED` включительно (`cancellableStatus` в `repository/receive/repository.go`); на схеме показаны три представителя. После `SIGNING` отзыв невозможен. Истечение оферты выполняет `offerexpiry` worker; ответ клиента после `expires_at` также переводит дело в `EXPIRED`. Recheck (`POST /backoffice/applications/{id}/recheck`) доступен назначенному аналитику, увеличивает `assessment_generation`, закрывает открытый запрос данных и повторяет конвейер с `VALIDATION`.

### Отображение для клиента

bank-service публикует клиенту только `ApplicationPublicStatePublished` с `ClientStatus` (`proto/events/events.proto`); client-service хранит его в `applications.status`. Скоринг, данные бюро и комментарии аналитика клиенту не передаются.

| Статус в bank_db | `ClientStatus` / `applications.status` | Событие публикует |
|---|---|---|
| — | `DRAFT` | только client-service |
| `SUBMITTED`, `VALIDATION` | `SUBMITTED` | client-service при отправке; банк ещё не публиковал состояние |
| `ENRICHMENT`, `SCORING`, `DECISION`, `MANUAL_REVIEW` | `UNDER_REVIEW` | validation (`UnderReview`), assessment и take (`ManualReview`/`UnderReview`), recheck |
| `NEEDS_INFO` | `NEEDS_INFO` | validation, assessment, решение `REQUEST_INFO` |
| `OFFER`, `COUNTER_OFFER` | `OFFER_AVAILABLE` | решение аналитика `APPROVE`/`COUNTER` |
| `OFFER_ACCEPTED`, `SIGNING` | `FORMALIZING` | ответ на оферту, подпись |
| `DISBURSED` | `DISBURSED` | disbursement worker |
| `REJECTED` | `REJECTED` | assessment или решение `REJECT` |
| `CANCELLED`, `EXPIRED` | `CLOSED` | отзыв, отказ от оферты, истечение |

Пока команда клиента не подтверждена банком, client-service не меняет последний подтверждённый статус, а ставит `pending_action`: `INFO_SENT`, `ACCEPT_PENDING`, `DECLINE_PENDING`, `SIGN_PENDING`, `CANCEL_PENDING`.

## Основной сквозной сценарий

```mermaid
sequenceDiagram
    autonumber
    actor C as Клиент
    participant CS as client-service
    participant K as Kafka
    participant BS as bank-service
    participant BM as bureau-mocks
    participant SC as scoring
    actor A as Аналитик

    C->>CS: POST /client/applications/{id}/submit
    CS->>CS: tx: status=SUBMITTED + outbox ApplicationSubmitted
    CS-->>K: relay → client.events
    K-->>BS: ApplicationSubmitted
    BS->>BS: tx: processed_events + cases VALIDATION + audit
    BS->>BS: validation worker → ENRICHMENT, outbox UNDER_REVIEW
    BS->>BM: GetCompanyProfile, GetCreditHistory, GetAssessmentSources
    BS->>BS: enrichment_results, assessment_sources → SCORING
    BS->>SC: Score (baseline v1)
    BS->>BS: scoring_attempts, score_results → DECISION
    BS->>BS: pkg/assessment Evaluate → assessments, MANUAL_REVIEW
    BS-->>K: outbox ApplicationPublicStatePublished UNDER_REVIEW + публичная оценка
    K-->>CS: bank.events → applications.status, public_assessment

    A->>BS: POST /backoffice/applications/{id}/take
    A->>BS: GET .../assessment, POST .../decision APPROVE или COUNTER
    BS->>BS: CheckCurrentOffer: условия = кандидат оценки
    BS->>BS: tx: decisions + offers + OFFER/COUNTER_OFFER + outbox
    BS-->>K: OFFER_AVAILABLE
    K-->>CS: проекция оферты

    C->>CS: POST .../offer/accept
    CS-->>K: OfferAccepted
    K-->>BS: OFFER_ACCEPTED, outbox FORMALIZING
    C->>CS: POST .../sign
    CS-->>K: ContractSigned
    K-->>BS: SIGNING, next_run_at = now + 3 s
    BS->>BS: disbursement worker → DISBURSED, audit DEMO_DISBURSED
    BS-->>K: DISBURSED
    K-->>CS: applications.status = DISBURSED
```

Каждый переход в bank-service выполняется одной транзакцией: обновление `cases` с проверкой статуса и версии, запись `audit_log` и, если меняется публичное состояние, строка `outbox`. Worker'ы захватывают дела через `FOR UPDATE SKIP LOCKED` и lease (`lease_until`), поэтому повторный запуск не дублирует переход.
