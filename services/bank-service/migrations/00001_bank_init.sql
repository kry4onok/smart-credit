-- +goose Up
-- Bank owns these tables. IDs from auth/client have no cross-database FKs.

CREATE TABLE products (
    id uuid NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    code text NOT NULL CHECK (code <> ''),
    name text NOT NULL CHECK (name <> ''),
    description text NOT NULL DEFAULT '',
    purpose_tags text[] NOT NULL CHECK (
        cardinality(purpose_tags) > 0 AND
        purpose_tags <@ ARRAY['WORKING_CAPITAL', 'EQUIPMENT', 'EXPANSION']::text[]
    ),
    required_document_types text[] NOT NULL DEFAULT '{}' CHECK (
        required_document_types <@ ARRAY['BANK_STATEMENT', 'BUSINESS_PLAN']::text[]
    ),
    min_amount_kopecks bigint NOT NULL CHECK (min_amount_kopecks > 0),
    max_amount_kopecks bigint NOT NULL CHECK (max_amount_kopecks >= min_amount_kopecks),
    min_term_months integer NOT NULL CHECK (min_term_months > 0),
    max_term_months integer NOT NULL CHECK (max_term_months >= min_term_months),
    indicative_rate_bps integer NOT NULL CHECK (indicative_rate_bps >= 0),
    is_active boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id, version),
    UNIQUE (code, version)
);

CREATE UNIQUE INDEX products_one_active_code_idx
    ON products (code)
WHERE is_active;

CREATE TABLE product_rules (
    product_id uuid NOT NULL,
    product_version bigint NOT NULL,
    min_company_age_months integer NOT NULL CHECK (min_company_age_months >= 0),
    max_amount_to_revenue_bps integer NOT NULL CHECK (max_amount_to_revenue_bps BETWEEN 0 AND 10000),
    max_overdue_days integer NOT NULL CHECK (max_overdue_days >= 0),
    rate_grade_a_bps integer NOT NULL CHECK (rate_grade_a_bps >= 0),
    rate_grade_b_bps integer NOT NULL CHECK (rate_grade_b_bps >= 0),
    offer_ttl_hours integer NOT NULL CHECK (offer_ttl_hours > 0),
    PRIMARY KEY (product_id, product_version),
    FOREIGN KEY (product_id, product_version) REFERENCES products (id, version)
);

CREATE TABLE cases (
    id uuid PRIMARY KEY, -- Equals the client application ID.
    org_id uuid NOT NULL,
    client_user_id uuid NOT NULL,
    product_id uuid NOT NULL, -- No FK: unknown product must remain reviewable.
    product_version bigint NOT NULL CHECK (product_version > 0),
    product_name text NOT NULL CHECK (product_name <> ''),
    company_inn text NOT NULL CHECK (company_inn ~ '^[0-9]{10}([0-9]{2})?$'),
    company_name text NOT NULL CHECK (company_name <> ''),
    company_snapshot jsonb NOT NULL CHECK (jsonb_typeof(company_snapshot) = 'object'),
    application_snapshot jsonb NOT NULL CHECK (jsonb_typeof(application_snapshot) = 'object'),
    documents_snapshot jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(documents_snapshot) = 'array'),
    requested_amount_kopecks bigint NOT NULL CHECK (requested_amount_kopecks > 0),
    requested_term_months integer NOT NULL CHECK (requested_term_months > 0),
    purpose_tag text NOT NULL CHECK (purpose_tag IN ('WORKING_CAPITAL', 'EQUIPMENT', 'EXPANSION')),
    status text NOT NULL DEFAULT 'SUBMITTED' CHECK (status IN (
        'SUBMITTED', 'VALIDATION', 'NEEDS_INFO', 'ENRICHMENT', 'SCORING',
        'DECISION', 'MANUAL_REVIEW', 'OFFER', 'COUNTER_OFFER', 'OFFER_ACCEPTED',
        'SIGNING', 'DISBURSED', 'REJECTED', 'CANCELLED', 'EXPIRED'
    )),
    recommendation text CHECK (recommendation IN (
        'APPROVE', 'COUNTER', 'REQUEST_INFO', 'MANUAL_REVIEW', 'REJECT'
    )),
    assignee_id uuid, -- Users live in auth_db.
    public_reason text,
    triggered_rules text[] NOT NULL DEFAULT '{}',
    next_run_at timestamptz,
    lease_until timestamptz,
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    audit_version bigint NOT NULL DEFAULT 0 CHECK (audit_version >= 0),
    client_event_version bigint NOT NULL CHECK (client_event_version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (updated_at >= created_at)
);

CREATE INDEX cases_queue_idx ON cases (
    (CASE WHEN status = 'MANUAL_REVIEW' THEN 0 ELSE 1 END), created_at, id
);

CREATE INDEX cases_status_idx
    ON cases (status, created_at, id);

CREATE INDEX cases_product_idx
    ON cases (product_id, created_at, id);

CREATE INDEX cases_assignee_idx
    ON cases (assignee_id, created_at, id)
WHERE assignee_id IS NOT NULL;

CREATE INDEX cases_due_idx
    ON cases (next_run_at, id)
WHERE next_run_at IS NOT NULL;

CREATE TABLE enrichment_results (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES cases (id),
    source text NOT NULL CHECK (source IN ('COMPANY', 'CREDIT_HISTORY')),
    outcome text NOT NULL CHECK (outcome IN ('SUCCESS', 'ERROR')),
    result jsonb,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((outcome = 'SUCCESS' AND jsonb_typeof(result) = 'object' AND error_code IS NULL)
        OR (outcome = 'ERROR' AND result IS NULL AND error_code IS NOT NULL))
);

CREATE INDEX enrichment_results_case_idx
    ON enrichment_results (case_id, created_at DESC);

CREATE TABLE score_results (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES cases (id),
    score integer NOT NULL CHECK (score BETWEEN 0 AND 1000),
    grade text NOT NULL CHECK (
        (grade = 'A' AND score BETWEEN 800 AND 1000) OR
        (grade = 'B' AND score BETWEEN 650 AND 799) OR
        (grade = 'C' AND score BETWEEN 500 AND 649) OR
        (grade = 'D' AND score BETWEEN 350 AND 499) OR
        (grade = 'E' AND score BETWEEN 0 AND 349)
    ),
    reasons jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(reasons) = 'array'),
    features jsonb NOT NULL CHECK (jsonb_typeof(features) = 'object'),
    model_version text NOT NULL CHECK (model_version <> ''),
    feature_schema_version text NOT NULL CHECK (feature_schema_version <> ''),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX score_results_case_idx
    ON score_results (case_id, created_at DESC);

CREATE TABLE decisions (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES cases (id),
    actor_type text NOT NULL CHECK (actor_type IN ('SYSTEM', 'ANALYST')),
    actor_id uuid,
    outcome text NOT NULL CHECK (outcome IN (
        'APPROVE', 'COUNTER', 'REQUEST_INFO', 'MANUAL_REVIEW', 'REJECT'
    )),
    internal_comment text,
    public_reason text,
    product_id uuid,
    product_version bigint,
    score_result_id bigint REFERENCES score_results (id),
    rule_version bigint,
    triggered_rules text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((actor_type = 'SYSTEM' AND actor_id IS NULL)
        OR (actor_type = 'ANALYST' AND actor_id IS NOT NULL AND internal_comment IS NOT NULL)),
    CHECK ((product_id IS NULL AND product_version IS NULL)
        OR (product_id IS NOT NULL AND product_version > 0))
);

CREATE INDEX decisions_case_idx
    ON decisions (case_id, created_at DESC);

CREATE TABLE info_requests (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES cases (id),
    decision_id uuid REFERENCES decisions (id),
    message text NOT NULL CHECK (message <> ''),
    required_fields text[] NOT NULL DEFAULT '{}',
    required_document_types text[] NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'FULFILLED', 'CANCELLED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    responded_at timestamptz,
    CHECK (cardinality(required_fields) + cardinality(required_document_types) > 0)
);

CREATE UNIQUE INDEX info_requests_one_open_idx
    ON info_requests (case_id)
WHERE status = 'OPEN';

CREATE TABLE offers (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES cases (id),
    decision_id uuid NOT NULL REFERENCES decisions (id),
    kind text NOT NULL CHECK (kind IN ('OFFER', 'COUNTER_OFFER')),
    product_id uuid NOT NULL,
    product_version bigint NOT NULL CHECK (product_version > 0),
    product_name text NOT NULL CHECK (product_name <> ''),
    amount_kopecks bigint NOT NULL CHECK (amount_kopecks > 0),
    term_months integer NOT NULL CHECK (term_months > 0),
    rate_bps integer NOT NULL CHECK (rate_bps >= 0),
    explanation text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN (
        'AVAILABLE', 'ACCEPTED', 'DECLINED', 'EXPIRED', 'SIGNED'
    )),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    accepted_at timestamptz,
    signed_at timestamptz,
    CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX offers_one_available_idx
    ON offers (case_id)
WHERE status = 'AVAILABLE';

CREATE INDEX offers_expiring_idx
    ON offers (expires_at, id)
WHERE status = 'AVAILABLE';

CREATE TABLE audit_log (
    id uuid PRIMARY KEY,
    case_id uuid NOT NULL REFERENCES cases (id),
    case_version bigint NOT NULL CHECK (case_version > 0),
    action text NOT NULL CHECK (action <> ''),
    actor_type text NOT NULL CHECK (actor_type IN ('SYSTEM', 'ANALYST', 'CLIENT')),
    actor_id uuid,
    reason text NOT NULL DEFAULT '',
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object'),
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_case_idx
    ON audit_log (case_id, occurred_at, id);

-- Store the complete serialized protobuf Event. The relay publishes it unchanged.
CREATE TABLE outbox (
    event_id uuid PRIMARY KEY,
    topic text NOT NULL DEFAULT 'bank.events' CHECK (topic = 'bank.events'),
    aggregate_id text NOT NULL CHECK (aggregate_id <> ''),
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    event_type text NOT NULL CHECK (event_type IN (
        'ApplicationPublicStatePublished', 'ProductCatalogPublished'
    )),
    payload bytea NOT NULL CHECK (octet_length(payload) > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    published_at timestamptz,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    UNIQUE (aggregate_id, aggregate_version)
);

CREATE INDEX outbox_pending_idx
    ON outbox (created_at, event_id)
WHERE published_at IS NULL;

-- Insert with the case change; acknowledge Kafka only after commit.
CREATE TABLE processed_events (
    event_id uuid PRIMARY KEY,
    topic text NOT NULL DEFAULT 'client.events' CHECK (topic = 'client.events'),
    aggregate_id uuid NOT NULL,
    aggregate_version bigint NOT NULL CHECK (aggregate_version > 0),
    processed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (topic, aggregate_id, aggregate_version)
);

CREATE TABLE idempotency_keys (
    actor_id uuid NOT NULL,
    route text NOT NULL CHECK (route <> ''),
    idempotency_key uuid NOT NULL,
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    response_status integer NOT NULL CHECK (response_status BETWEEN 200 AND 599),
    response_body jsonb NOT NULL,
    response_headers jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(response_headers) = 'object'),
    case_id uuid REFERENCES cases (id),
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
DROP TABLE audit_log;
DROP TABLE offers;
DROP TABLE info_requests;
DROP TABLE decisions;
DROP TABLE score_results;
DROP TABLE enrichment_results;
DROP TABLE cases;
DROP TABLE product_rules;
DROP TABLE products;
