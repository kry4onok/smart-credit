package disbursementrepository

const claimDueSigningQuery = `
SELECT id, version, clock_timestamp()
FROM cases
WHERE status = 'SIGNING' AND next_run_at <= now()
ORDER BY next_run_at, id
LIMIT 1
FOR UPDATE SKIP LOCKED`

const completeDisbursementQuery = `
UPDATE cases
SET status = 'DISBURSED', version = version + 1,
    audit_version = audit_version + 1, next_run_at = NULL,
    lease_until = NULL, updated_at = $3
WHERE id = $1 AND status = 'SIGNING' AND version = $2
RETURNING version`

const insertDisbursementAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, reason, details, occurred_at)
VALUES ($1, $2, $3, 'DEMO_DISBURSED', 'SYSTEM', 'Демо-выдача после подписания договора',
    jsonb_build_object('from_status', 'SIGNING', 'to_status', 'DISBURSED',
                       'reason', 'DEMO_DISBURSEMENT'), $4)`

const insertDisbursementOutboxQuery = `
INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
VALUES ($1, $2, $3, 'ApplicationPublicStatePublished', $4)`
