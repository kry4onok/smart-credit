package validationrepository

const claimNextQuery = `
SELECT id, version, product_id, product_version, company_snapshot,
       documents_snapshot, requirements_reference_date,
       requested_amount_kopecks, requested_term_months, purpose_tag
FROM cases
WHERE status = 'VALIDATION'
  AND (next_run_at IS NULL OR next_run_at <= now())
ORDER BY created_at, id
FOR UPDATE SKIP LOCKED
LIMIT 1`

const loadProductQuery = `
SELECT COALESCE(requirements_policy_version, ''), purpose_tags, required_document_types,
       min_amount_kopecks, max_amount_kopecks, min_term_months, max_term_months
FROM products
WHERE id = $1 AND version = $2`

const finishCaseQuery = `
UPDATE cases
SET status = $3,
    recommendation = $4,
    triggered_rules = $5,
    public_reason = $6,
    next_run_at = CASE WHEN $3 = 'ENRICHMENT' THEN now() ELSE NULL END,
    version = version + 1,
    audit_version = audit_version + 1,
    updated_at = now()
WHERE id = $1 AND version = $2 AND status = 'VALIDATION'
RETURNING version`

const insertInfoRequestQuery = `
INSERT INTO info_requests (id, case_id, message, required_fields, required_document_types)
VALUES ($1, $2, $3, $4, $5)`

const insertV2InfoRequestQuery = `
INSERT INTO info_requests (id, case_id, message, requirements)
VALUES ($1, $2, $3, $4::jsonb)`

const insertAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, reason, details)
VALUES ($1, $2, $3, 'VALIDATION_COMPLETED', 'SYSTEM', $4, $5::jsonb)`
