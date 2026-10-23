package enrichmentrepository

const claimNextQuery = `
WITH candidate AS (
    SELECT id FROM cases
    WHERE status = 'ENRICHMENT'
      AND (next_run_at IS NULL OR next_run_at <= now())
      AND (lease_until IS NULL OR lease_until < now())
    ORDER BY created_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE cases AS c
SET lease_until = now() + interval '30 seconds'
FROM candidate
WHERE c.id = candidate.id
RETURNING c.id, c.company_inn, c.version, c.scoring_input_revision, c.lease_until`

const finishCaseQuery = `
UPDATE cases
SET status = $5,
    recommendation = $6,
    scoring_input_revision = scoring_input_revision + 1,
    current_scoring_attempt_id = NULL,
    triggered_rules = '{}',
    public_reason = NULL,
    lease_until = NULL,
    next_run_at = NULL,
    version = version + 1,
    audit_version = audit_version + 1,
    updated_at = now()
WHERE id = $1 AND lease_until = $2 AND version = $3
  AND scoring_input_revision = $4 AND status = 'ENRICHMENT'
RETURNING version`

const insertResultQuery = `
INSERT INTO enrichment_results (case_id, source, outcome, result, error_code)
VALUES ($1, $2, $3, $4::jsonb, $5)`

const insertAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, reason, details)
VALUES ($1, $2, $3, 'ENRICHMENT_COMPLETED', 'SYSTEM', $4, $5::jsonb)`

const releaseQuery = `
UPDATE cases
SET lease_until = NULL, next_run_at = now() + interval '5 seconds'
WHERE id = $1 AND lease_until = $2 AND status = 'ENRICHMENT'`
