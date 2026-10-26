# Модель данных

Схемы построены по итоговому состоянию goose-миграций: `services/auth/migrations/00001`, `services/client-service/migrations/00001–00003`, `services/bank-service/migrations/00001–00007`. Показаны ключи, статусы, версии, деньги и связи; полный список колонок и `CHECK` — в миграциях.

Общие правила:

* три БД в одном PostgreSQL 16, у каждой свой владелец (`auth_user`, `client_user`, `bank_user`, см. `deploy/postgres/init-dbs.sh`);
* межбазовых внешних ключей нет; ID пользователей, организаций и заявок совпадают по значению, но целостность между БД обеспечивают события;
* деньги — `bigint` в копейках (`*_kopecks`), ставки — `integer` в базисных пунктах (`*_bps`);
* оптимистическая блокировка — колонка `version`; порядок событий — `aggregate_version`, `client_event_version`, `bank_event_version`;
* типы массивов на схемах записаны как `text_array`.

## auth_db

Владелец — `auth`. Только учётные записи и сессии; финансовых данных нет. Пароли хранятся как Argon2id PHC, refresh-токены — только SHA-256. Роль `CLIENT` обязана иметь `org_id`, `ANALYST`/`ADMIN` — нет.

```mermaid
erDiagram
    organizations ||--o{ users : "org_id"
    users ||--o{ refresh_tokens : "user_id"

    organizations {
        uuid id PK
        text inn UK "10 или 12 цифр"
        text name
    }
    users {
        uuid id PK
        text email UK "lower(email)"
        text password_hash "argon2id"
        text role "CLIENT, ANALYST, ADMIN"
        uuid org_id FK "только CLIENT"
        boolean can_submit
        boolean is_active
    }
    refresh_tokens {
        bytea token_hash PK "sha256"
        uuid user_id FK
        timestamptz expires_at "не более 7 дней"
        timestamptz revoked_at
    }
```

## client_db

Владелец — `client-service`. Источник истины для профиля компании, черновика, загруженных файлов и действий клиента. Таблицы `products`, `info_requests`, `offers` и `applications.status` — проекции из `bank.events`; клиентская часть не видит скоринг, бюро и комментарии аналитика. Содержимое файлов лежит в volume по `storage_key`, в БД — метаданные и SHA-256.

```mermaid
erDiagram
    companies ||--o{ applications : "org_id"
    applications ||--o{ application_documents : "application_id"
    applications ||--o{ info_requests : "application_id"
    applications ||--o{ offers : "application_id"
    applications ||--o{ timeline_entries : "application_id"
    applications ||--o{ outbox : "aggregate_id"
    applications |o--o{ idempotency_keys : "application_id"

    companies {
        uuid org_id PK "из auth_db, без FK"
        text inn
        bigint revenue_12m_kopecks "nullable с v2"
        bigint profit_12m_kopecks
        bigint obligations_kopecks
        integer profile_schema_version "2 или NULL"
        text legal_form "IP, LLC"
        jsonb tax_periods
        jsonb financial_period
        text financial_basis
        bigint version
    }
    products {
        uuid id PK "проекция каталога банка"
        bigint version
        text code UK
        bigint min_amount_kopecks
        bigint max_amount_kopecks
        integer indicative_rate_bps
        text_array required_document_types
        text requirements_policy_version
    }
    applications {
        uuid id PK "= cases.id в bank_db"
        uuid org_id FK
        uuid client_user_id
        uuid product_id "без FK"
        bigint product_version
        jsonb product_snapshot
        jsonb company_snapshot
        bigint requested_amount_kopecks
        integer requested_term_months
        text purpose_tag
        text status "DRAFT ... CLOSED"
        text pending_action "INFO_SENT, ACCEPT_PENDING, ..."
        date requirements_reference_date
        jsonb bank_requirements_plan
        jsonb assessment_input "Submission v1"
        jsonb public_assessment "PublicResult"
        bigint version
        bigint client_event_version
        bigint bank_event_version
    }
    application_documents {
        uuid id PK
        uuid application_id FK
        text document_type
        text storage_key UK "не отдаётся наружу"
        bigint size_bytes "до 5 MiB"
        text sha256
        boolean is_mock "всегда true"
        text requirement_id
        date coverage_from
        date coverage_to
    }
    info_requests {
        uuid id PK "id запроса банка"
        uuid application_id FK
        text_array required_fields
        text_array required_document_types
        jsonb requirements "v2"
        boolean is_active "один активный"
    }
    offers {
        uuid id PK "id оферты банка"
        uuid application_id FK
        text kind "OFFER, COUNTER_OFFER"
        bigint amount_kopecks
        integer term_months
        integer rate_bps
        timestamptz expires_at
        boolean is_active "одна активная"
    }
    timeline_entries {
        uuid id PK
        uuid application_id FK
        text status
        text actor_type "SYSTEM, CLIENT"
        uuid event_id
    }
    outbox {
        uuid event_id PK
        text topic "client.events"
        uuid aggregate_id FK
        bigint aggregate_version UK
        text event_type
        bytea payload "protobuf Event"
        timestamptz published_at
    }
    processed_events {
        uuid event_id PK
        text topic "bank.events"
        text aggregate_id "id заявки или catalog"
        bigint aggregate_version UK
    }
    idempotency_keys {
        uuid actor_id PK
        text route PK
        uuid idempotency_key PK
        bytea request_hash
        jsonb response_body
        uuid application_id FK
        timestamptz expires_at
    }
```

## bank_db

Владелец — `bank-service`. Источник истины для продуктов и правил, дела (`cases.id` равен ID клиентской заявки), обогащения, скоринга, оценки v2, решений, оферт и аудита. Клиентские данные попадают сюда только как снимки (`company_snapshot`, `application_snapshot`, `documents_snapshot`) из событий `client.events`. `cases.product_id` намеренно без FK, чтобы дело с неизвестным продуктом попадало на ручной разбор.

```mermaid
erDiagram
    products ||--|| product_rules : "id, version"
    cases ||--o{ enrichment_results : "case_id"
    cases ||--o{ scoring_attempts : "case_id"
    cases |o--o| scoring_attempts : "current_scoring_attempt_id"
    scoring_attempts |o--o| score_results : "attempt_id"
    cases ||--o{ score_results : "case_id"
    cases ||--o{ assessments : "case_id"
    cases ||--o{ assessment_commands : "case_id"
    cases ||--o{ decisions : "case_id"
    score_results |o--o{ decisions : "score_result_id"
    decisions ||--o{ offers : "decision_id"
    decisions |o--o{ info_requests : "decision_id"
    cases ||--o{ offers : "case_id"
    cases ||--o{ info_requests : "case_id"
    cases ||--o{ audit_log : "case_id"
    cases |o--o{ idempotency_keys : "case_id"

    products {
        uuid id PK
        bigint version PK
        text code
        bigint min_amount_kopecks
        bigint max_amount_kopecks
        integer indicative_rate_bps
        text_array required_document_types
        text requirements_policy_version "NULL, sme-profile-v1, sme-profile-v2"
        boolean is_active "одна активная версия"
    }
    product_rules {
        uuid product_id PK, FK
        bigint product_version PK, FK
        integer max_amount_to_revenue_bps
        integer max_overdue_days
        integer rate_grade_a_bps
        integer rate_grade_b_bps
        integer offer_ttl_hours
    }
    cases {
        uuid id PK "= applications.id"
        uuid org_id
        uuid client_user_id
        uuid product_id "без FK"
        bigint product_version
        text company_inn
        jsonb company_snapshot
        jsonb documents_snapshot
        bigint requested_amount_kopecks
        integer requested_term_months
        text status "VALIDATION ... EXPIRED"
        text recommendation
        uuid assignee_id "из auth_db"
        text_array triggered_rules
        timestamptz next_run_at
        timestamptz lease_until
        bigint version
        bigint audit_version
        bigint client_event_version
        bigint scoring_input_revision
        uuid current_scoring_attempt_id FK
        bigint assessment_generation
        jsonb assessment_input
        jsonb assessment_sources
    }
    enrichment_results {
        bigint id PK
        uuid case_id FK
        text source "COMPANY, CREDIT_HISTORY"
        text outcome "SUCCESS, ERROR"
        bigint assessment_generation
    }
    scoring_attempts {
        uuid id PK
        uuid case_id FK
        bigint input_revision
        text status "PENDING, ASSESSED, ..."
        jsonb request
        jsonb response
        bigint assessment_generation
    }
    score_results {
        bigint id PK
        uuid case_id FK
        uuid attempt_id FK, UK
        integer score "0..1000"
        text grade "A..E"
        text model_version
    }
    assessments {
        text id PK
        uuid case_id FK
        bigint input_revision
        bigint assessment_generation UK
        jsonb result "полный, для банка"
        jsonb public_result "allowlist для клиента"
        timestamptz valid_until
    }
    assessment_commands {
        uuid case_id PK, FK
        uuid idempotency_key PK
        uuid actor_id
        bigint expected_version
        bigint generation
    }
    decisions {
        uuid id PK
        uuid case_id FK
        text actor_type "SYSTEM, ANALYST"
        uuid actor_id
        text outcome "APPROVE, COUNTER, REQUEST_INFO, MANUAL_REVIEW, REJECT"
        bigint score_result_id FK
        text internal_comment "обязателен у ANALYST"
    }
    info_requests {
        uuid id PK
        uuid case_id FK
        uuid decision_id FK
        text status "OPEN, FULFILLED, CANCELLED"
        jsonb requirements
    }
    offers {
        uuid id PK
        uuid case_id FK
        uuid decision_id FK
        text kind "OFFER, COUNTER_OFFER"
        bigint amount_kopecks
        integer term_months
        integer rate_bps
        text status "AVAILABLE, ACCEPTED, DECLINED, EXPIRED, SIGNED"
        timestamptz expires_at
    }
    audit_log {
        uuid id PK
        uuid case_id FK
        bigint case_version
        text action
        text actor_type "SYSTEM, ANALYST, CLIENT"
        jsonb details
    }
    outbox {
        uuid event_id PK
        text topic "bank.events"
        text aggregate_id "id дела или catalog"
        bigint aggregate_version UK
        text event_type "ApplicationPublicStatePublished, ProductCatalogPublished"
        bytea payload
        timestamptz published_at
    }
    processed_events {
        uuid event_id PK
        text topic "client.events"
        uuid aggregate_id
        bigint aggregate_version UK
    }
    idempotency_keys {
        uuid actor_id PK
        text route PK
        uuid idempotency_key PK
        uuid case_id FK
    }
    catalog_publication {
        boolean singleton PK
        bigint version
        bytea content_hash
    }
```

Частичные уникальные индексы закрепляют инварианты: одна активная версия продукта (`products_one_active_code_idx`, `products_one_active_id_idx`), один открытый запрос данных (`info_requests_one_open_idx`), одна доступная оферта (`offers_one_available_idx`) на дело; в client_db — один активный запрос и одна активная оферта на заявку.
