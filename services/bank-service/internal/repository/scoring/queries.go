package scoringrepository

const lockCaseRevisionQuery = `
SELECT c.scoring_input_revision, a.status
FROM cases AS c
LEFT JOIN scoring_attempts AS a ON a.id = c.current_scoring_attempt_id
WHERE c.id = $1
FOR UPDATE OF c`

const loadInputQuery = `
SELECT c.product_id, c.scoring_input_revision, c.company_snapshot,
       company_check.outcome, company_check.result,
       credit_check.outcome, credit_check.result
FROM cases AS c
LEFT JOIN LATERAL (
    SELECT outcome, result FROM enrichment_results
    WHERE case_id = c.id AND source = 'COMPANY'
    ORDER BY created_at DESC, id DESC LIMIT 1
) AS company_check ON true
LEFT JOIN LATERAL (
    SELECT outcome, result FROM enrichment_results
    WHERE case_id = c.id AND source = 'CREDIT_HISTORY'
    ORDER BY created_at DESC, id DESC LIMIT 1
) AS credit_check ON true
WHERE c.id = $1`

const insertAttemptQuery = `
INSERT INTO scoring_attempts (id, case_id, input_revision, as_of_date, request)
VALUES ($1, $2, $3, $4::date, $5::jsonb)`

const selectCurrentAttemptQuery = `
UPDATE cases
SET current_scoring_attempt_id = $1,
    version = version + 1,
    updated_at = now()
WHERE id = $2`

const invalidateInputQuery = `
UPDATE cases
SET scoring_input_revision = scoring_input_revision + 1,
    current_scoring_attempt_id = NULL,
    version = version + 1,
    updated_at = now()
WHERE id = $1 AND scoring_input_revision = $2
RETURNING scoring_input_revision`

const lockAttemptQuery = `
SELECT case_id, input_revision, status
FROM scoring_attempts
WHERE id = $1
FOR UPDATE`

const completeAttemptQuery = `
UPDATE scoring_attempts
SET status = $2,
    response = $3::jsonb,
    error_code = $4,
    completed_at = now()
WHERE id = $1`

const currentAttemptQuery = `
SELECT COALESCE(current_scoring_attempt_id = $1, false)
    AND scoring_input_revision = $2
FROM cases
WHERE id = $3
FOR UPDATE`

const advanceCaseVersionQuery = `
UPDATE cases
SET version = version + 1, updated_at = now()
WHERE id = $1`

const insertAssessedScoreQuery = `
INSERT INTO score_results (
    case_id, score, grade, reasons, features, model_version,
    feature_schema_version, attempt_id
)
SELECT case_id, $2, $3, $4::jsonb, request->'features', $5, $6, id
FROM scoring_attempts
WHERE id = $1`

const claimScoringCaseQuery = `
WITH candidate AS (
    SELECT id FROM cases
    WHERE status = 'SCORING'
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
RETURNING c.id, c.lease_until`

const currentWorkerAttemptQuery = `
SELECT a.id, a.status,
       COALESCE(a.created_at < now() - interval '30 seconds', false)
FROM cases AS c
LEFT JOIN scoring_attempts AS a ON a.id = c.current_scoring_attempt_id
WHERE c.id = $1 AND c.status = 'SCORING' AND c.lease_until = $2`

const lockWorkerCaseQuery = `
SELECT c.current_scoring_attempt_id, a.status, a.error_code
FROM cases AS c
LEFT JOIN scoring_attempts AS a ON a.id = c.current_scoring_attempt_id
WHERE c.id = $1 AND c.status = 'SCORING' AND c.lease_until = $2
FOR UPDATE OF c`

const finishScoringCaseQuery = `
UPDATE cases
SET status = $3,
    recommendation = $4,
    lease_until = NULL,
    next_run_at = NULL,
    version = version + 1,
    audit_version = audit_version + 1,
    updated_at = now()
WHERE id = $1 AND lease_until = $2 AND status = 'SCORING'
RETURNING version`

const insertWorkerAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, reason, details)
VALUES ($1, $2, $3, 'SCORING_COMPLETED', 'SYSTEM', $4, $5::jsonb)`

const releaseScoringCaseQuery = `
UPDATE cases
SET lease_until = NULL, next_run_at = now() + interval '5 seconds'
WHERE id = $1 AND lease_until = $2 AND status = 'SCORING'`
