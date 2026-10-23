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
