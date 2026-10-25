package receiverepository

const selectAcceptedOfferQuery = `
SELECT id, accepted_at, clock_timestamp()
FROM offers
WHERE case_id = $1 AND status = 'ACCEPTED' AND accepted_at IS NOT NULL
ORDER BY accepted_at DESC, id DESC
LIMIT 1
FOR UPDATE`

const signAcceptedOfferQuery = `
UPDATE offers SET status = 'SIGNED', signed_at = $3
WHERE id = $1 AND case_id = $2 AND status = 'ACCEPTED' AND accepted_at <= $3`

const updateCaseForSigningQuery = `
UPDATE cases
SET status = 'SIGNING', client_event_version = $3, version = version + 1,
    audit_version = audit_version + 1, next_run_at = $5, lease_until = NULL,
    updated_at = $4
WHERE id = $1 AND status = 'OFFER_ACCEPTED' AND version = $2
  AND client_event_version = $3 - 1
RETURNING version`

const insertSigningAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, actor_id, reason, details, occurred_at)
VALUES ($1, $2, $3, 'CONTRACT_SIGNED', 'CLIENT', $4, 'Договор подписан клиентом',
    jsonb_build_object('from_status', 'OFFER_ACCEPTED', 'to_status', 'SIGNING',
                       'event_id', $5::text, 'offer_id', $6::text,
                       'signed_at', $7::timestamptz), $8)`
