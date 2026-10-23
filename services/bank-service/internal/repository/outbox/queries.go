package outboxrepository

const claimNextQuery = `
WITH candidate AS (
    SELECT pending.event_id
    FROM outbox AS pending
    WHERE pending.published_at IS NULL
      AND (pending.lease_until IS NULL OR pending.lease_until < now())
      AND NOT EXISTS (
          SELECT 1 FROM outbox AS earlier
          WHERE earlier.aggregate_id = pending.aggregate_id
            AND earlier.aggregate_version < pending.aggregate_version
            AND earlier.published_at IS NULL
      )
    ORDER BY pending.created_at, pending.event_id
    FOR UPDATE OF pending SKIP LOCKED
    LIMIT 1
)
UPDATE outbox AS current
SET lease_until = now() + interval '30 seconds',
    attempts = current.attempts + 1
FROM candidate
WHERE current.event_id = candidate.event_id
RETURNING current.event_id, current.topic, current.aggregate_id,
          current.aggregate_version, current.payload, current.lease_until,
          current.attempts`

const markPublishedQuery = `
UPDATE outbox
SET published_at = now(), lease_until = NULL
WHERE event_id = $1 AND lease_until = $2 AND published_at IS NULL`

const releaseQuery = `
UPDATE outbox
SET lease_until = NULL
WHERE event_id = $1 AND lease_until = $2 AND published_at IS NULL`
