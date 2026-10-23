package scoringrepository

const lockCaseRevisionQuery = `
SELECT scoring_input_revision
FROM cases
WHERE id = $1
FOR UPDATE`

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
