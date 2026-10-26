package offerexpiryrepository

const claimExpiredOfferQuery = `
SELECT c.id, c.version, o.id, clock_timestamp()
FROM offers o
JOIN cases c ON c.id = o.case_id
WHERE o.status = 'AVAILABLE' AND o.expires_at <= clock_timestamp()
  AND c.status IN ('OFFER', 'COUNTER_OFFER')
ORDER BY o.expires_at, o.id
LIMIT 1
FOR UPDATE OF c SKIP LOCKED`

const expireOfferQuery = `
UPDATE offers SET status = 'EXPIRED'
WHERE id = $1 AND case_id = $2 AND status = 'AVAILABLE' AND expires_at <= $3`

const expireCaseQuery = `
UPDATE cases
SET status = 'EXPIRED', version = version + 1, audit_version = audit_version + 1,
    next_run_at = NULL, lease_until = NULL, updated_at = $3
WHERE id = $1 AND version = $2 AND status IN ('OFFER', 'COUNTER_OFFER')
RETURNING version`

const insertExpiryAuditQuery = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, reason, details, occurred_at)
VALUES ($1, $2, $3, 'OFFER_EXPIRED', 'SYSTEM', 'OFFER_TTL_EXPIRED',
    jsonb_build_object('offer_id', $4::text, 'to_status', 'EXPIRED'), $5)`
