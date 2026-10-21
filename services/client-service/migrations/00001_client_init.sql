-- +goose Up
-- Organization/user IDs belong to auth_db; no cross-database foreign keys.
-- Money is stored in RUB kopecks within the exact integer range of the HTTP API.
CREATE TABLE companies (
    org_id uuid PRIMARY KEY,
    inn text NOT NULL CHECK (inn ~ '^[0-9]{10}([0-9]{2})?$'),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 256),
    okved text NOT NULL CHECK (char_length(okved) BETWEEN 2 AND 16),
    registered_at date NOT NULL,
    revenue_12m_kopecks bigint NOT NULL CHECK (revenue_12m_kopecks BETWEEN 0 AND 9007199254740991),
    profit_12m_kopecks bigint NOT NULL CHECK (profit_12m_kopecks BETWEEN -9007199254740991 AND 9007199254740991),
    obligations_kopecks bigint NOT NULL CHECK (obligations_kopecks BETWEEN 0 AND 9007199254740991),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (updated_at >= created_at)
);

-- Current public catalog only. Underwriting rules belong to bank-service.
CREATE TABLE products (
    id uuid PRIMARY KEY,
    version bigint NOT NULL CHECK (version > 0),
    code text NOT NULL UNIQUE CHECK (code <> ''),
    name text NOT NULL CHECK (name <> ''),
    description text NOT NULL DEFAULT '',
    purpose_tags text[] NOT NULL CHECK (
        cardinality(purpose_tags) > 0 AND
        purpose_tags <@ ARRAY['WORKING_CAPITAL', 'EQUIPMENT', 'EXPANSION']::text[] AND
        array_position(purpose_tags, NULL) IS NULL
    ),
    min_amount_kopecks bigint NOT NULL CHECK (min_amount_kopecks BETWEEN 1 AND 9007199254740991),
    max_amount_kopecks bigint NOT NULL CHECK (max_amount_kopecks BETWEEN min_amount_kopecks AND 9007199254740991),
    min_term_months integer NOT NULL CHECK (min_term_months > 0),
    max_term_months integer NOT NULL CHECK (max_term_months >= min_term_months),
    indicative_rate_bps integer CHECK (indicative_rate_bps >= 0),
    required_document_types text[] NOT NULL DEFAULT '{}' CHECK (
        required_document_types <@ ARRAY['BANK_STATEMENT', 'BUSINESS_PLAN']::text[] AND
        array_position(required_document_types, NULL) IS NULL
    ),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE applications (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES companies (org_id),
    client_user_id uuid NOT NULL,
    -- No catalog FK: replacing the current catalog must preserve old applications.
    product_id uuid NOT NULL,
    product_version bigint NOT NULL CHECK (product_version > 0),
    product_snapshot jsonb NOT NULL CHECK (jsonb_typeof(product_snapshot) = 'object'),
    company_snapshot jsonb NOT NULL CHECK (jsonb_typeof(company_snapshot) = 'object'),
    requested_amount_kopecks bigint CHECK (requested_amount_kopecks BETWEEN 1 AND 9007199254740991),
    requested_term_months integer CHECK (requested_term_months > 0),
    purpose_tag text CHECK (purpose_tag IN ('WORKING_CAPITAL', 'EQUIPMENT', 'EXPANSION')),
    status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN (
        'DRAFT', 'SUBMITTED', 'UNDER_REVIEW', 'NEEDS_INFO', 'OFFER_AVAILABLE',
        'FORMALIZING', 'DISBURSED', 'REJECTED', 'CLOSED'
    )),
    stage_label text NOT NULL DEFAULT 'Черновик',
    next_action text NOT NULL DEFAULT 'Заполните заявку',
    public_reason text,
    -- Pending commands do not overwrite the last confirmed bank status.
    pending_action text CHECK (pending_action IN (
        'INFO_SENT', 'ACCEPT_PENDING', 'DECLINE_PENDING', 'SIGN_PENDING', 'CANCEL_PENDING'
    )),
    pending_since timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    client_event_version bigint NOT NULL DEFAULT 0 CHECK (client_event_version >= 0),
    bank_event_version bigint NOT NULL DEFAULT 0 CHECK (bank_event_version >= 0),
    submitted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (updated_at >= created_at),
    CHECK ((pending_action IS NULL) = (pending_since IS NULL)),
    CHECK (status <> 'DRAFT' OR pending_action IS NULL),
    CHECK (status = 'DRAFT' OR (
        requested_amount_kopecks IS NOT NULL AND requested_term_months IS NOT NULL AND
        purpose_tag IS NOT NULL AND submitted_at IS NOT NULL AND client_event_version > 0
    ))
);

CREATE INDEX applications_org_created_idx
    ON applications (org_id, created_at, id);

CREATE INDEX applications_org_status_created_idx
    ON applications (org_id, status, created_at, id);

CREATE TABLE application_documents (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES applications (id),
    document_type text NOT NULL CHECK (document_type IN ('BANK_STATEMENT', 'BUSINESS_PLAN')),
    file_name text NOT NULL CHECK (btrim(file_name) <> ''),
    -- Internal volume key; never include it in HTTP responses or bank events.
    storage_key text NOT NULL UNIQUE CHECK (storage_key <> ''),
    size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 0 AND 5242880),
    sha256 text NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    is_mock boolean NOT NULL DEFAULT true CHECK (is_mock),
    uploaded_by uuid NOT NULL,
    uploaded_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX application_documents_application_idx
    ON application_documents (application_id, uploaded_at, id);

-- Public projections. Deactivate previous rows and upsert the active snapshot
-- in the same transaction as applications.bank_event_version and processed_events.
CREATE TABLE info_requests (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES applications (id),
    message text NOT NULL CHECK (message <> ''),
    required_fields text[] NOT NULL DEFAULT '{}' CHECK (
        required_fields <@ ARRAY[
            'COMPANY_REVENUE_12M', 'COMPANY_PROFIT_12M', 'COMPANY_OBLIGATIONS',
            'APPLICATION_AMOUNT', 'APPLICATION_TERM_MONTHS', 'APPLICATION_PURPOSE'
        ]::text[] AND array_position(required_fields, NULL) IS NULL
    ),
    required_document_types text[] NOT NULL DEFAULT '{}' CHECK (
        required_document_types <@ ARRAY['BANK_STATEMENT', 'BUSINESS_PLAN']::text[] AND
        array_position(required_document_types, NULL) IS NULL
    ),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL,
    responded_at timestamptz,
    CHECK (cardinality(required_fields) + cardinality(required_document_types) > 0),
    CHECK (responded_at IS NULL OR responded_at >= created_at)
);

CREATE UNIQUE INDEX info_requests_one_active_idx
    ON info_requests (application_id) WHERE is_active;

CREATE INDEX info_requests_application_idx
    ON info_requests (application_id, created_at, id);

CREATE TABLE offers (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES applications (id),
    kind text NOT NULL CHECK (kind IN ('OFFER', 'COUNTER_OFFER')),
    -- Offered product may differ from the requested product and current catalog.
    product_id uuid NOT NULL,
    product_version bigint NOT NULL CHECK (product_version > 0),
    product_name text NOT NULL CHECK (product_name <> ''),
    amount_kopecks bigint NOT NULL CHECK (amount_kopecks BETWEEN 1 AND 9007199254740991),
    term_months integer NOT NULL CHECK (term_months > 0),
    rate_bps integer NOT NULL CHECK (rate_bps >= 0),
    explanation text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    received_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX offers_one_active_idx
    ON offers (application_id) WHERE is_active;

CREATE INDEX offers_application_idx
    ON offers (application_id, received_at, id);

-- Only client-visible messages; no score, bureau data or analyst comments.
CREATE TABLE timeline_entries (
    id uuid PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES applications (id),
    status text NOT NULL CHECK (status IN (
        'DRAFT', 'SUBMITTED', 'UNDER_REVIEW', 'NEEDS_INFO', 'OFFER_AVAILABLE',
        'FORMALIZING', 'DISBURSED', 'REJECTED', 'CLOSED'
    )),
    action text NOT NULL CHECK (action <> ''),
    actor_type text NOT NULL CHECK (actor_type IN ('SYSTEM', 'CLIENT')),
    actor_id uuid,
    message text NOT NULL CHECK (message <> ''),
    event_id uuid,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((actor_type = 'SYSTEM' AND actor_id IS NULL)
        OR (actor_type = 'CLIENT' AND actor_id IS NOT NULL))
);

CREATE INDEX timeline_entries_application_idx
    ON timeline_entries (application_id, occurred_at, id);

-- Complete serialized protobuf Event, including correlation_id and occurred_at.
-- Relay must not publish a newer aggregate version before the older one succeeds.
CREATE TABLE outbox (
    event_id uuid PRIMARY KEY,
    topic text NOT NULL DEFAULT 'client.events' CHECK (topic = 'client.events'),
    aggregate_id uuid NOT NULL REFERENCES applications (id),
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    event_type text NOT NULL CHECK (event_type IN (
        'ApplicationSubmitted', 'InfoProvided', 'OfferAccepted',
        'OfferDeclined', 'ContractSigned', 'ApplicationCancelled'
    )),
    payload bytea NOT NULL CHECK (octet_length(payload) > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    UNIQUE (aggregate_id, aggregate_version)
);

CREATE INDEX outbox_pending_idx
    ON outbox (created_at, event_id) WHERE published_at IS NULL;

CREATE INDEX outbox_aggregate_pending_idx
    ON outbox (aggregate_id, aggregate_version) WHERE published_at IS NULL;

-- aggregate_id is text because catalog events use the literal "catalog".
-- The latest processed catalog version is MAX(aggregate_version) for "catalog".
-- Retain that record during cleanup, even if the published catalog was empty.
CREATE TABLE processed_events (
    event_id uuid PRIMARY KEY,
    topic text NOT NULL DEFAULT 'bank.events' CHECK (topic = 'bank.events'),
    aggregate_id text NOT NULL CHECK (aggregate_id <> ''),
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    processed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (topic, aggregate_id, aggregate_version)
);

-- Save the result in the same transaction as the business operation.
-- route includes the resource ID to distinguish actions on different applications.
CREATE TABLE idempotency_keys (
    actor_id uuid NOT NULL,
    route text NOT NULL CHECK (route <> ''),
    idempotency_key uuid NOT NULL,
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    response_status integer NOT NULL CHECK (response_status BETWEEN 200 AND 599),
    response_body jsonb NOT NULL,
    response_headers jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(response_headers) = 'object'),
    application_id uuid REFERENCES applications (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (actor_id, route, idempotency_key),
    CHECK (expires_at > created_at)
);

CREATE INDEX idempotency_keys_expires_idx ON idempotency_keys (expires_at);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TABLE processed_events;
DROP TABLE outbox;
DROP TABLE timeline_entries;
DROP TABLE offers;
DROP TABLE info_requests;
DROP TABLE application_documents;
DROP TABLE applications;
DROP TABLE products;
DROP TABLE companies;
