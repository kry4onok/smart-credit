package caserepository

const decisionRoute = "POST /backoffice/applications/{application_id}/decision"

const reserveDecisionKey = `
INSERT INTO idempotency_keys (actor_id,route,idempotency_key,request_hash,response_status,response_body,expires_at)
VALUES ($1,$2,$3,$4,200,'{}'::jsonb,now()+interval '24 hours')
ON CONFLICT (actor_id,route,idempotency_key) DO UPDATE SET
 request_hash=EXCLUDED.request_hash,response_status=200,response_body='{}'::jsonb,
 response_headers='{}'::jsonb,case_id=NULL,created_at=now(),expires_at=now()+interval '24 hours'
WHERE idempotency_keys.expires_at<=now()
RETURNING 1`

const readDecisionKey = `
SELECT request_hash,response_body 
FROM idempotency_keys 
WHERE actor_id=$1 AND route=$2 AND idempotency_key=$3 
FOR UPDATE`

const saveDecisionKey = `
UPDATE idempotency_keys 
SET response_body=$4,response_headers=jsonb_build_object('ETag',$5::text),case_id=$6 
WHERE actor_id=$1 AND route=$2 AND idempotency_key=$3`

const lockDecisionCase = `
SELECT status,assignee_id,version,company_snapshot,documents_snapshot,purpose_tag 
FROM cases 
WHERE id=$1 
FOR UPDATE`

const loadDecisionProduct = `
SELECT p.version,p.name,p.purpose_tags,p.required_document_types,p.min_amount_kopecks,p.max_amount_kopecks,
	p.min_term_months,p.max_term_months,r.rate_grade_a_bps,r.rate_grade_b_bps,
	r.min_company_age_months,r.max_amount_to_revenue_bps,r.max_overdue_days,r.offer_ttl_hours
FROM products p 
    JOIN product_rules r 
        ON r.product_id=p.id AND r.product_version=p.version
WHERE p.id=$1 AND p.is_active 
ORDER BY p.version DESC 
LIMIT 1`

const latestDecisionOverdue = `
SELECT CASE WHEN outcome='SUCCESS' 
    THEN (result->>'max_overdue_days')::integer END 
FROM enrichment_results 
WHERE case_id=$1 AND source='CREDIT_HISTORY' 
ORDER BY created_at DESC,id DESC 
LIMIT 1`

const latestDecisionCompanyStatus = `
SELECT CASE WHEN outcome='SUCCESS' THEN result->>'status' END 
FROM enrichment_results 
WHERE case_id=$1 AND source='COMPANY' 
ORDER BY created_at DESC,id DESC 
LIMIT 1`

const latestDecisionGrade = `
SELECT s.grade 
FROM cases c JOIN score_results s 
    ON s.case_id=c.id AND ((c.current_scoring_attempt_id IS NOT NULL AND s.attempt_id=c.current_scoring_attempt_id) OR (c.current_scoring_attempt_id IS NULL AND c.scoring_input_revision=1 AND s.attempt_id IS NULL))
WHERE c.id=$1 
ORDER BY s.created_at DESC,s.id DESC 
LIMIT 1`

const updateDecisionCase = `
UPDATE cases SET status=$2,recommendation=$3,public_reason=$4,triggered_rules='{}',version=version+1,audit_version=audit_version+1,next_run_at=NULL,lease_until=NULL,updated_at=now() 
WHERE id=$1`

const insertDecision = `
INSERT INTO decisions (id,case_id,actor_type,actor_id,outcome,internal_comment,public_reason,product_id,product_version,rule_version) 
VALUES ($1,$2,'ANALYST',$3,$4,$5,$6,$7,$8,$9)`

const insertDecisionOffer = `
INSERT INTO offers (id,case_id,decision_id,kind,product_id,product_version,product_name,amount_kopecks,term_months,rate_bps,explanation,expires_at) 
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

const insertDecisionInfo = `
INSERT INTO info_requests (id,case_id,decision_id,message,required_fields,required_document_types) 
VALUES ($1,$2,$3,$4,$5,$6)`

const insertDecisionAudit = `
INSERT INTO audit_log (id,case_id,case_version,action,actor_type,actor_id,reason,details) 
VALUES ($1,$2,$3,'DECISION','ANALYST',$4,$5,$6)`

const insertDecisionOutbox = `
INSERT INTO outbox (event_id,aggregate_id,aggregate_version,event_type,payload) 
VALUES ($1,$2,$3,'ApplicationPublicStatePublished',$4)`
