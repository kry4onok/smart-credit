package disbursementrepository

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
	var completed bool
	err := r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		q := transaction.QueryerFromContext(ctx, r.db)
		var caseID uuid.UUID
		var version int64
		var now time.Time
		err := q.QueryRow(ctx, claimDueSigningQuery).Scan(&caseID, &version, &now)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim signing case: %w", err)
		}
		var newVersion int64
		err = q.QueryRow(ctx, completeDisbursementQuery, caseID, version, now).Scan(&newVersion)
		if err != nil {
			return fmt.Errorf("complete demo disbursement: %w", err)
		}
		if _, err := q.Exec(ctx, insertDisbursementAuditQuery, uuid.New(), caseID, newVersion, now); err != nil {
			return fmt.Errorf("audit demo disbursement: %w", err)
		}
		eventID := uuid.New()
		payload, err := publicstate.Disbursed(eventID, caseID, newVersion, now)
		if err != nil {
			return fmt.Errorf("encode disbursed state: %w", err)
		}
		if _, err := publicstate.Publish(ctx, q, eventID,
			caseID.String(), newVersion, payload); err != nil {
			return fmt.Errorf("write disbursed outbox: %w", err)
		}
		completed = true
		return nil
	})
	return completed && err == nil, err
}
