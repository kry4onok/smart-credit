package caserepository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/take"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const takeRoute = "POST /backoffice/applications/{application_id}/take"

const insertInKeys = `
INSERT INTO idempotency_keys (actor_id, route, idempotency_key, request_hash,
response_status, response_body, expires_at)
VALUES ($1, $2, $3, $4, 200, '{}'::jsonb, now() + interval '24 hours')
	ON CONFLICT (actor_id, route, idempotency_key) DO UPDATE SET
		request_hash = EXCLUDED.request_hash,
		response_status = 200,
		response_body = '{}'::jsonb,
		response_headers = '{}'::jsonb,
		case_id = NULL,
		created_at = now(),
		expires_at = now() + interval '24 hours'
WHERE idempotency_keys.expires_at <= now()
RETURNING 1`

const selectResp = `
SELECT request_hash, response_body
FROM idempotency_keys
WHERE actor_id=$1 AND route=$2 AND idempotency_key=$3 
FOR UPDATE`

const selectStatus = `
SELECT status, assignee_id, version
FROM cases
WHERE id=$1
FOR UPDATE`

const updateCases = `
UPDATE cases SET 
	assignee_id=$2, 
	version=version+1,
    audit_version=audit_version+1, 
	updated_at=now()
WHERE id=$1`

const insertAuditLog = `
INSERT INTO audit_log (id, case_id, case_version, action, actor_type, actor_id)
VALUES ($1, $2, $3, 'TAKE', 'ANALYST', $4)`

const insertOutbox = `
INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
VALUES ($1, $2, $3, 'ApplicationPublicStatePublished', $4)`

const updateIdempotencyKeys = `
UPDATE idempotency_keys
SET response_body=$4,
    response_headers=jsonb_build_object('ETag', $5::text),
    case_id=$6
WHERE actor_id=$1 AND route=$2 AND idempotency_key=$3`

func (r *Repository) Take(ctx context.Context, input take.Input) (model.Case, error) {
	q := transaction.QueryerFromContext(ctx, r.db)
	hash := sha256.Sum256([]byte(input.CaseID.String() + ":" + strconv.FormatInt(input.ExpectedVersion, 10)))
	var reserved int
	err := q.QueryRow(
		ctx,
		insertInKeys,
		input.AnalystID,
		takeRoute,
		input.IdempotencyKey,
		hash[:],
	).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {

		var existingHash []byte
		var body []byte

		err = q.QueryRow(
			ctx,
			selectResp,
			input.AnalystID,
			takeRoute,
			input.IdempotencyKey,
		).Scan(&existingHash, &body)
		if err != nil {
			return model.Case{}, fmt.Errorf("read idempotency response: %w", err)
		}
		if string(existingHash) != string(hash[:]) {
			return model.Case{}, take.ErrIdempotencyConflict
		}

		var result model.Case

		if err := json.Unmarshal(body, &result); err != nil {
			return model.Case{}, fmt.Errorf("decode idempotency response: %w", err)
		}

		return result, nil
	}
	if err != nil {
		return model.Case{}, fmt.Errorf("reserve idempotency key: %w", err)
	}

	var status model.CaseStatus
	var assignee *uuid.UUID
	var version int64

	err = q.QueryRow(
		ctx,
		selectStatus,
		input.CaseID,
	).Scan(&status, &assignee, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Case{}, take.ErrNotFound
	}
	if err != nil {
		return model.Case{}, fmt.Errorf("lock case: %w", err)
	}
	if version != input.ExpectedVersion || status != model.CaseStatusManualReview || assignee != nil {
		return model.Case{}, take.ErrConflict
	}

	_, err = q.Exec(
		ctx,
		updateCases,
		input.CaseID,
		input.AnalystID,
	)
	if err != nil {
		return model.Case{}, fmt.Errorf("assign case: %w", err)
	}

	_, err = q.Exec(
		ctx,
		insertAuditLog,
		uuid.New(),
		input.CaseID,
		version+1,
		input.AnalystID,
	)
	if err != nil {
		return model.Case{}, fmt.Errorf("audit take: %w", err)
	}
	eventID := uuid.New()
	payload := publicstate.UnderReview(eventID, input.CaseID, input.IdempotencyKey, version+1, time.Now().UTC())
	_, err = q.Exec(ctx, insertOutbox, eventID, input.CaseID.String(), version+1, payload)
	if err != nil {
		return model.Case{}, fmt.Errorf("write take outbox event: %w", err)
	}

	result, found, err := r.GetCase(ctx, input.CaseID)
	if err != nil {
		return model.Case{}, err
	}
	if !found {
		return model.Case{}, take.ErrNotFound
	}

	body, err := json.Marshal(result)
	if err != nil {
		return model.Case{}, fmt.Errorf("encode idempotency response: %w", err)
	}

	_, err = q.Exec(
		ctx,
		updateIdempotencyKeys,
		input.AnalystID,
		takeRoute,
		input.IdempotencyKey,
		body,
		fmt.Sprintf("\"%d\"", result.Version),
		input.CaseID,
	)
	if err != nil {
		return model.Case{}, fmt.Errorf("save idempotency response: %w", err)
	}

	return result, nil
}
