package offerexpiryrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db           transaction.Queryer
	transactions *transaction.Manager
}

func New(db transaction.Queryer, transactions *transaction.Manager) *Repository {
	return &Repository{db: db, transactions: transactions}
}

func (r *Repository) RunOnce(ctx context.Context) (bool, error) {
	var expired bool
	err := r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		q := transaction.QueryerFromContext(ctx, r.db)
		var caseID, offerID uuid.UUID
		var version int64
		var now time.Time
		err := q.QueryRow(ctx, claimExpiredOfferQuery).Scan(&caseID, &version, &offerID, &now)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim expired offer: %w", err)
		}
		result, err := q.Exec(ctx, expireOfferQuery, offerID, caseID, now)
		if err != nil {
			return fmt.Errorf("expire offer: %w", err)
		}
		if result.RowsAffected() == 0 {
			return nil
		}
		var newVersion int64
		err = q.QueryRow(ctx, expireCaseQuery, caseID, version, now).Scan(&newVersion)
		if err != nil {
			return fmt.Errorf("expire case: %w", err)
		}
		if _, err := q.Exec(ctx, insertExpiryAuditQuery, uuid.New(), caseID, newVersion, offerID, now); err != nil {
			return fmt.Errorf("audit offer expiry: %w", err)
		}
		eventID := uuid.New()
		payload, err := publicstate.OfferResponse(eventID, caseID, newVersion, now, "OfferExpired")
		if err != nil {
			return fmt.Errorf("encode offer expiry: %w", err)
		}
		if _, err := q.Exec(ctx, insertExpiryOutboxQuery, eventID, caseID.String(), newVersion, payload); err != nil {
			return fmt.Errorf("write offer expiry outbox: %w", err)
		}
		expired = true
		return nil
	})
	return expired && err == nil, err
}
