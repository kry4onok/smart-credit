package decisionrepository

const claimCaseQuery = `
SELECT id, version, product_id, product_version, company_snapshot, documents_snapshot, requirements_reference_date,
       requested_amount_kopecks, requested_term_months, purpose_tag
FROM cases
WHERE status = 'DECISION' AND (next_run_at IS NULL OR next_run_at <= now())
ORDER BY created_at, id
FOR UPDATE SKIP LOCKED
LIMIT 1`

const loadProductQuery = `
SELECT p.name, p.version, p.purpose_tags, p.required_document_types,
       p.min_amount_kopecks, p.max_amount_kopecks, p.min_term_months, p.max_term_months,
       r.min_company_age_months, r.max_amount_to_revenue_bps, r.max_overdue_days,
       r.rate_grade_a_bps, r.rate_grade_b_bps, r.offer_ttl_hours, COALESCE(p.requirements_policy_version,'')
FROM products p JOIN product_rules r ON r.product_id=p.id AND r.product_version=p.version
WHERE p.id=$1 AND p.version=$2 AND p.is_active`

const loadScoreQuery = `
SELECT s.id, s.grade
FROM score_results s JOIN cases c ON c.id=s.case_id
WHERE c.id=$1 AND (
    (c.current_scoring_attempt_id IS NOT NULL AND s.attempt_id=c.current_scoring_attempt_id)
    OR (c.current_scoring_attempt_id IS NULL AND c.scoring_input_revision=1 AND s.attempt_id IS NULL)
)
ORDER BY s.created_at DESC,s.id DESC LIMIT 1`

const loadChecksQuery = `
SELECT company.outcome, company.result->>'status',
       credit.outcome, (credit.result->>'max_overdue_days')::integer
FROM cases c
LEFT JOIN LATERAL (
    SELECT outcome,result FROM enrichment_results
    WHERE case_id=c.id AND source='COMPANY'
    ORDER BY created_at DESC,id DESC LIMIT 1
) company ON true
LEFT JOIN LATERAL (
    SELECT outcome,result FROM enrichment_results
    WHERE case_id=c.id AND source='CREDIT_HISTORY'
    ORDER BY created_at DESC,id DESC LIMIT 1
) credit ON true
WHERE c.id=$1`

const insertDecisionQuery = `
INSERT INTO decisions (id,case_id,actor_type,outcome,public_reason,product_id,product_version,
                       score_result_id,rule_version,triggered_rules)
VALUES ($1,$2,'SYSTEM',$3,$4,$5,$6,$7,$8,$9)`

const insertOfferQuery = `
INSERT INTO offers (id,case_id,decision_id,kind,product_id,product_version,product_name,
                    amount_kopecks,term_months,rate_bps,explanation,expires_at)
VALUES ($1,$2,$3,'OFFER',$4,$5,$6,$7,$8,$9,$10,$11)`

const insertInfoRequestQuery = `
INSERT INTO info_requests (id,case_id,decision_id,message,required_fields,required_document_types)
VALUES ($1,$2,$3,$4,$5,$6)`

const insertV2InfoRequestQuery = `
INSERT INTO info_requests (id,case_id,decision_id,message,requirements)
VALUES ($1,$2,$3,$4,$5::jsonb)`

const finishCaseQuery = `
UPDATE cases SET status=$3,recommendation=$4,triggered_rules=$5,public_reason=$6,
                 next_run_at=NULL,lease_until=NULL,version=version+1,
                 audit_version=audit_version+1,updated_at=now()
WHERE id=$1 AND version=$2 AND status='DECISION'
RETURNING version`

const insertAuditQuery = `
INSERT INTO audit_log (id,case_id,case_version,action,actor_type,reason,details)
VALUES ($1,$2,$3,'DECISION_COMPLETED','SYSTEM',$4,$5::jsonb)`

const insertOutboxQuery = `
INSERT INTO outbox (event_id,aggregate_id,aggregate_version,event_type,payload)
VALUES ($1,$2,$3,'ApplicationPublicStatePublished',$4)`
