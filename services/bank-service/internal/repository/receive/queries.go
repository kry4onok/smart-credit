package receiverepository

const reserveEventQuery = `
INSERT INTO processed_events (event_id, aggregate_id, aggregate_version)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING
RETURNING 1`

const selectDuplicateEventQuery = `
SELECT aggregate_id, aggregate_version
FROM processed_events
WHERE event_id = $1`

const selectProductQuery = `
SELECT name
FROM products
WHERE id = $1 AND version = $2`

const insertCaseQuery = `
INSERT INTO cases (
    id, org_id, client_user_id, product_id, product_version, product_name,
    company_inn, company_name, company_snapshot, application_snapshot,
    documents_snapshot, requested_amount_kopecks, requested_term_months,
    purpose_tag, status, recommendation, triggered_rules, next_run_at,
    version, audit_version, client_event_version
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9::jsonb, $10::jsonb,
    $11::jsonb, $12, $13,
    $14, $15, $16, $17,
    CASE WHEN $18::boolean THEN now() ELSE NULL END,
    1, 1, $19
)`

const insertAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, actor_id, reason, details)
VALUES ($1, $2, 1, 'APPLICATION_SUBMITTED', 'CLIENT', $3, $4,
        jsonb_build_object('event_id', $5::text))`

const selectCaseForInfoQuery = `
SELECT status, client_event_version
FROM cases
WHERE id = $1
FOR UPDATE`

const selectOpenInfoRequestQuery = `
SELECT id
FROM info_requests
WHERE case_id = $1 AND status = 'OPEN'
FOR UPDATE`

const fulfillInfoRequestQuery = `
UPDATE info_requests
SET status = 'FULFILLED', responded_at = now()
WHERE id = $1 AND case_id = $2 AND status = 'OPEN'`

const updateCaseFromInfoQuery = `
UPDATE cases
SET company_inn = $2, company_name = $3,
    company_snapshot = $4::jsonb, application_snapshot = $5::jsonb,
    documents_snapshot = $6::jsonb, requested_amount_kopecks = $7,
    requested_term_months = $8, purpose_tag = $9,
    status = 'VALIDATION', recommendation = NULL, public_reason = NULL,
    triggered_rules = '{}', next_run_at = now(), lease_until = NULL,
    scoring_input_revision = scoring_input_revision + 1,
    current_scoring_attempt_id = NULL,
    client_event_version = $10, version = version + 1,
    audit_version = audit_version + 1, updated_at = now()
WHERE id = $1 AND status = 'NEEDS_INFO' AND client_event_version < $10
RETURNING version`

const insertInfoProvidedAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, details)
VALUES ($1, $2, $3, 'INFO_PROVIDED', 'CLIENT',
        jsonb_build_object('event_id', $4::text, 'request_id', $5::text))`

const selectCaseForOfferQuery = `
SELECT status, version, client_event_version, client_user_id
FROM cases WHERE id = $1 FOR UPDATE`

const updateCaseForCancellationQuery = `
UPDATE cases
SET status = 'CANCELLED', client_event_version = $3, version = version + 1,
    audit_version = audit_version + 1, next_run_at = NULL, lease_until = NULL,
    updated_at = clock_timestamp()
WHERE id = $1 AND status = $2 AND version = $4 AND client_event_version = $3 - 1
RETURNING version`

const cancelOpenInfoRequestQuery = `
UPDATE info_requests SET status = 'CANCELLED'
WHERE case_id = $1 AND status = 'OPEN'`

const declineAvailableOfferQuery = `
UPDATE offers SET status = 'DECLINED'
WHERE case_id = $1 AND status = 'AVAILABLE'`

const selectCancellationTimeQuery = `SELECT clock_timestamp()`

const insertCancellationAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, actor_id, details, occurred_at)
VALUES ($1, $2, $3, 'APPLICATION_CANCELLED', 'CLIENT', $4,
    jsonb_build_object('from_status', $5::text, 'to_status', 'CANCELLED',
                       'event_id', $6::text, 'reason', $7::text), $8)`

const selectCurrentOfferQuery = `
SELECT id, expires_at, clock_timestamp()
FROM offers
WHERE case_id = $1 AND status = 'AVAILABLE'
FOR UPDATE`

const updateOfferResponseQuery = `
UPDATE offers SET status = $3, accepted_at = CASE WHEN $3 = 'ACCEPTED' THEN $4::timestamptz ELSE NULL END
WHERE id = $1 AND case_id = $2 AND status = 'AVAILABLE' AND expires_at > clock_timestamp()`

const expireOfferResponseQuery = `
UPDATE offers SET status = $3
WHERE id = $1 AND case_id = $2 AND status = 'AVAILABLE' AND expires_at <= $4`

const updateCaseForOfferQuery = `
UPDATE cases
SET status = $3, client_event_version = $4, version = version + 1,
    audit_version = audit_version + 1, next_run_at = NULL, lease_until = NULL,
    updated_at = $6
WHERE id = $1 AND status = $2 AND version = $5 AND client_event_version = $4 - 1
RETURNING version`

const insertOfferResponseAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, actor_id, details, occurred_at)
VALUES ($1, $2, $3, $4, 'CLIENT', $5,
    jsonb_build_object('from_status', $6::text, 'to_status', $7::text,
                       'event_id', $8::text, 'offer_id', $9::text), $10)`

const insertOfferResponseOutboxQuery = `
INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
VALUES ($1, $2, $3, 'ApplicationPublicStatePublished', $4)`
