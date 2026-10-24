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

const acceptOfferRoutePrefix = "POST /client/applications/"

func (r *Repository) AcceptOffer(ctx context.Context, params model.AcceptOfferParams) (model.ActionAccepted, error) {
	route := acceptOfferRoutePrefix + params.ApplicationID.String() + "/offer/accept"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("begin offer acceptance: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reserved int
	err = tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (
			actor_id, route, idempotency_key, request_hash, response_status,
			response_body, application_id, expires_at
		) VALUES ($1, $2, $3, $4, 202, '{}'::jsonb, $5, now() + interval '24 hours')
		ON CONFLICT (actor_id, route, idempotency_key) DO NOTHING
		RETURNING 1`, params.UserID, route, params.IdempotencyID, params.RequestHash, params.ApplicationID).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var storedHash []byte
		var responseBody []byte
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_body
			FROM idempotency_keys
			WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3
			FOR UPDATE`, params.UserID, route, params.IdempotencyID).Scan(&storedHash, &responseBody)
		if err != nil {
			return model.ActionAccepted{}, fmt.Errorf("load offer acceptance idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, params.RequestHash) {
			return model.ActionAccepted{}, domain.ErrIdempotencyConflict
		}
		var replay model.ActionAccepted
		if err := json.Unmarshal(responseBody, &replay); err != nil {
			return model.ActionAccepted{}, fmt.Errorf("decode offer acceptance idempotency response: %w", err)
		}
		return replay, nil
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("reserve offer acceptance idempotency key: %w", err)
	}

	now := time.Now().UTC()
	var status string
	var pendingAction *string
	var version, clientEventVersion int64
	err = tx.QueryRow(ctx, `
		SELECT status, pending_action, version, client_event_version
		FROM applications
		WHERE id = $1 AND org_id = $2
		FOR UPDATE`, params.ApplicationID, params.OrgID).Scan(
		&status, &pendingAction, &version, &clientEventVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ActionAccepted{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("load application for offer acceptance: %w", err)
	}
	if status != "OFFER_AVAILABLE" || pendingAction != nil {
		return model.ActionAccepted{}, domain.ErrApplicationOfferConflict
	}

	var activeOfferID uuid.UUID
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, expires_at
		FROM offers
		WHERE application_id = $1 AND is_active
		ORDER BY received_at DESC, id DESC
		LIMIT 1
		FOR UPDATE`, params.ApplicationID).Scan(&activeOfferID, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ActionAccepted{}, domain.ErrApplicationOfferConflict
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("load active offer: %w", err)
	}
	if activeOfferID != params.OfferID || !expiresAt.After(now) {
		return model.ActionAccepted{}, domain.ErrApplicationOfferConflict
	}

	eventID := uuid.New()
	clientEventVersion++
	event := &eventspb.Event{
		EventId:          eventID.String(),
		AggregateType:    "APPLICATION",
		AggregateId:      params.ApplicationID.String(),
		AggregateVersion: clientEventVersion,
		OccurredAt:       timestamppb.New(now),
		CorrelationId:    eventID.String(),
		Payload: &eventspb.Event_OfferAccepted{OfferAccepted: &eventspb.OfferAccepted{
			OfferId: params.OfferID.String(),
		}},
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("marshal offer accepted event: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE applications
		SET pending_action = 'ACCEPT_PENDING', pending_since = $3,
		    version = $4, client_event_version = $5, updated_at = $3
		WHERE id = $1 AND org_id = $2 AND status = 'OFFER_AVAILABLE' AND pending_action IS NULL`,
		params.ApplicationID, params.OrgID, now, version+1, clientEventVersion)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save offer acceptance: %w", err)
	}
	if result.RowsAffected() != 1 {
		return model.ActionAccepted{}, domain.ErrApplicationOfferConflict
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO timeline_entries (
			id, application_id, status, action, actor_type, actor_id, message, event_id, occurred_at
		) VALUES ($1, $2, 'OFFER_AVAILABLE', 'ACCEPT_OFFER', 'CLIENT', $3,
		          'Принятие оферты отправлено в банк', $4, $5)`,
		uuid.New(), params.ApplicationID, params.UserID, eventID, now); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("create offer acceptance timeline: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
		VALUES ($1, $2, $3, 'OfferAccepted', $4)`,
		eventID, params.ApplicationID, clientEventVersion, payload); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save offer accepted outbox event: %w", err)
	}

	accepted := model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "ACCEPT_PENDING"}
	responseBody, err := json.Marshal(accepted)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("encode offer acceptance response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $4
		WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
		params.UserID, route, params.IdempotencyID, responseBody); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save offer acceptance idempotency response: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("commit offer acceptance: %w", err)
	}
	return accepted, nil
}
