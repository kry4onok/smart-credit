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

func (r *Repository) SignContract(ctx context.Context, params model.SignContractParams) (model.ActionAccepted, error) {
	route := "POST /client/applications/" + params.ApplicationID.String() + "/sign"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("begin contract signing: %w", err)
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
			return model.ActionAccepted{}, fmt.Errorf("load contract signing idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, params.RequestHash) {
			return model.ActionAccepted{}, domain.ErrIdempotencyConflict
		}
		var replay model.ActionAccepted
		if err := json.Unmarshal(responseBody, &replay); err != nil {
			return model.ActionAccepted{}, fmt.Errorf("decode contract signing idempotency response: %w", err)
		}
		return replay, nil
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("reserve contract signing idempotency key: %w", err)
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
		return model.ActionAccepted{}, fmt.Errorf("load application for contract signing: %w", err)
	}
	if status != "FORMALIZING" || pendingAction != nil {
		return model.ActionAccepted{}, domain.ErrApplicationSignConflict
	}

	var offerID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM offers
		WHERE application_id = $1
		ORDER BY received_at DESC, id DESC
		LIMIT 1
		FOR UPDATE`, params.ApplicationID).Scan(&offerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ActionAccepted{}, domain.ErrApplicationSignConflict
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("load offer for contract signing: %w", err)
	}
	if offerID != params.OfferID {
		return model.ActionAccepted{}, domain.ErrApplicationSignConflict
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
		Payload: &eventspb.Event_ContractSigned{ContractSigned: &eventspb.ContractSigned{
			OfferId: params.OfferID.String(), SignedAt: timestamppb.New(now),
		}},
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("marshal contract signed event: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE applications
		SET pending_action = 'SIGN_PENDING', pending_since = $3,
		    version = $4, client_event_version = $5, updated_at = greatest($3, created_at)
		WHERE id = $1 AND org_id = $2 AND status = 'FORMALIZING' AND pending_action IS NULL`,
		params.ApplicationID, params.OrgID, now, version+1, clientEventVersion)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save contract signing action: %w", err)
	}
	if result.RowsAffected() != 1 {
		return model.ActionAccepted{}, domain.ErrApplicationSignConflict
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO timeline_entries (
			id, application_id, status, action, actor_type, actor_id, message, event_id, occurred_at
		) VALUES ($1, $2, 'FORMALIZING', 'SIGN', 'CLIENT', $3,
		          'Подписание договора отправлено в банк', $4, $5)`,
		uuid.New(), params.ApplicationID, params.UserID, eventID, now); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("create contract signing timeline: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
		VALUES ($1, $2, $3, 'ContractSigned', $4)`,
		eventID, params.ApplicationID, clientEventVersion, payload); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save contract signing outbox event: %w", err)
	}

	accepted := model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "SIGN_PENDING"}
	responseBody, err := json.Marshal(accepted)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("encode contract signing response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $4, application_id = $5
		WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
		params.UserID, route, params.IdempotencyID, responseBody, params.ApplicationID); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save contract signing idempotency response: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("commit contract signing: %w", err)
	}
	return accepted, nil
}
