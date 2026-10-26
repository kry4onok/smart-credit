package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (r *Repository) CancelApplication(ctx context.Context, params model.CancelApplicationParams) (model.ActionAccepted, error) {
	route := "POST /client/applications/" + params.ApplicationID.String() + "/cancel"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("begin application cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reserved int
	err = tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (
			actor_id, route, idempotency_key, request_hash, response_status,
			response_body, application_id, expires_at
		) VALUES ($1, $2, $3, $4, 202, '{}'::jsonb, NULL, now() + interval '24 hours')
		ON CONFLICT (actor_id, route, idempotency_key) DO NOTHING
		RETURNING 1`, params.UserID, route, params.IdempotencyID, params.RequestHash).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var storedHash []byte
		var responseBody []byte
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_body
			FROM idempotency_keys
			WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3
			FOR UPDATE`, params.UserID, route, params.IdempotencyID).Scan(&storedHash, &responseBody)
		if err != nil {
			return model.ActionAccepted{}, fmt.Errorf("load application cancellation idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, params.RequestHash) {
			return model.ActionAccepted{}, domain.ErrIdempotencyConflict
		}
		var replay model.ActionAccepted
		if err := json.Unmarshal(responseBody, &replay); err != nil {
			return model.ActionAccepted{}, fmt.Errorf("decode application cancellation idempotency response: %w", err)
		}
		return replay, nil
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("reserve application cancellation idempotency key: %w", err)
	}

	var status string
	var pendingAction *string
	var version, clientEventVersion int64
	err = tx.QueryRow(ctx, `
		SELECT status, pending_action, version, client_event_version
		FROM applications
		WHERE id = $1 AND org_id = $2
		FOR UPDATE`, params.ApplicationID, params.OrgID).Scan(&status, &pendingAction, &version, &clientEventVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ActionAccepted{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("load application for cancellation: %w", err)
	}
	var hasSignature bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM timeline_entries
			WHERE application_id = $1 AND action = 'SIGN'
		)`, params.ApplicationID).Scan(&hasSignature); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("check application signature before cancellation: %w", err)
	}
	if !domain.CanCancelApplication(status, pendingAction, hasSignature) {
		return model.ActionAccepted{}, domain.ErrApplicationCancelConflict
	}

	now := time.Now().UTC()
	eventID := uuid.New()
	clientEventVersion++
	event := &eventspb.Event{
		EventId:          eventID.String(),
		AggregateType:    "APPLICATION",
		AggregateId:      params.ApplicationID.String(),
		AggregateVersion: clientEventVersion,
		OccurredAt:       timestamppb.New(now),
		CorrelationId:    eventID.String(),
		Payload: &eventspb.Event_ApplicationCancelled{ApplicationCancelled: &eventspb.ApplicationCancelled{
			Reason: params.Reason,
		}},
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("marshal application cancelled event: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE applications
		SET pending_action = 'CANCEL_PENDING', pending_since = $3,
		    version = $4, client_event_version = $5, updated_at = greatest($3, created_at)
		WHERE id = $1 AND org_id = $2 AND status = $6 AND pending_action IS NULL`,
		params.ApplicationID, params.OrgID, now, version+1, clientEventVersion, status)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save application cancellation action: %w", err)
	}
	if result.RowsAffected() != 1 {
		return model.ActionAccepted{}, domain.ErrApplicationCancelConflict
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO timeline_entries (
			id, application_id, status, action, actor_type, actor_id, message, event_id, occurred_at
		) VALUES ($1, $2, $3, 'CANCEL', 'CLIENT', $4,
		          'Отзыв заявки отправлен в банк', $5, $6)`,
		uuid.New(), params.ApplicationID, status, params.UserID, eventID, now); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("create application cancellation timeline: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
		VALUES ($1, $2, $3, 'ApplicationCancelled', $4)`,
		eventID, params.ApplicationID, clientEventVersion, payload); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save application cancellation outbox event: %w", err)
	}

	accepted := model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "CANCEL_PENDING"}
	responseBody, err := json.Marshal(accepted)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("encode application cancellation response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $4, application_id = $5
		WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
		params.UserID, route, params.IdempotencyID, responseBody, params.ApplicationID); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save application cancellation idempotency response: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("commit application cancellation: %w", err)
	}
	return accepted, nil
}
