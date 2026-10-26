package decisionrepository

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
