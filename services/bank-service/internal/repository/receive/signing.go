package receiverepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const demoDisbursementDelay = 3 * time.Second
const maxFutureSignatureSkew = 5 * time.Minute

func (r *Repository) applyContractSigned(ctx context.Context, q transaction.Queryer, input receive.Input) (bool, error) {
	var status string
	var caseVersion, clientEventVersion int64
	var clientUserID uuid.UUID
	err := q.QueryRow(
		ctx,
		selectCaseForOfferQuery,
		input.ApplicationID,
	).Scan(&status, &caseVersion, &clientEventVersion, &clientUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrSigningConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock case for signing: %w", err)
	}
	if status != "OFFER_ACCEPTED" || input.AggregateVersion != clientEventVersion+1 {
		return false, ErrSigningConflict
	}
	var offerID uuid.UUID
	var acceptedAt, now time.Time
	err = q.QueryRow(
		ctx,
		selectAcceptedOfferQuery,
		input.ApplicationID,
	).Scan(&offerID, &acceptedAt, &now)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrSigningConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock accepted offer: %w", err)
	}
	if offerID != input.OfferID || input.SignedAt.Before(acceptedAt) ||
		input.SignedAt.After(now.Add(maxFutureSignatureSkew)) {
		return false, ErrSigningConflict
	}
	result, err := q.Exec(
		ctx,
		signAcceptedOfferQuery,
		input.OfferID,
		input.ApplicationID,
		input.SignedAt,
	)
	if err != nil {
		return false, fmt.Errorf("sign accepted offer: %w", err)
	}
	if result.RowsAffected() != 1 {
		return false, ErrSigningConflict
	}
	var newVersion int64
	err = q.QueryRow(
		ctx,
		updateCaseForSigningQuery,
		input.ApplicationID,
		caseVersion,
		input.AggregateVersion,
		now,
		now.Add(demoDisbursementDelay),
	).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrSigningConflict
	}
	if err != nil {
		return false, fmt.Errorf("move case to signing: %w", err)
	}
	if _, err := q.Exec(ctx, insertSigningAuditQuery, uuid.New(), input.ApplicationID,
		newVersion, clientUserID, input.EventID, input.OfferID, input.SignedAt, now); err != nil {
		return false, fmt.Errorf("audit contract signing: %w", err)
	}
	eventID := uuid.New()
	payload, err := publicstate.Signing(eventID, input.ApplicationID, newVersion, now)
	if err != nil {
		return false, fmt.Errorf("encode signing state: %w", err)
	}
	if _, err := q.Exec(ctx, insertOfferResponseOutboxQuery, eventID,
		input.ApplicationID.String(), newVersion, payload); err != nil {
		return false, fmt.Errorf("write signing outbox: %w", err)
	}
	return true, nil
}
