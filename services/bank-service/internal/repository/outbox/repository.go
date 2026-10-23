package outboxrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Event struct {
	ID               uuid.UUID
	Topic            string
	AggregateID      string
	AggregateVersion int64
	Payload          []byte
	LeaseUntil       time.Time
	Attempts         int
}

type Repository struct {
	db transaction.Queryer
}

func New(db transaction.Queryer) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ClaimNext(ctx context.Context) (Event, bool, error) {
	var event Event
	err := r.db.QueryRow(ctx, claimNextQuery).Scan(
		&event.ID, &event.Topic, &event.AggregateID, &event.AggregateVersion,
		&event.Payload, &event.LeaseUntil, &event.Attempts,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, fmt.Errorf("claim outbox event: %w", err)
	}
	return event, true, nil
}

func (r *Repository) MarkPublished(ctx context.Context, event Event) (bool, error) {
	result, err := r.db.Exec(ctx, markPublishedQuery, event.ID, event.LeaseUntil)
	if err != nil {
		return false, fmt.Errorf("mark outbox event published: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (r *Repository) Release(ctx context.Context, event Event) error {
	_, err := r.db.Exec(ctx, releaseQuery, event.ID, event.LeaseUntil)
	if err != nil {
		return fmt.Errorf("release outbox event: %w", err)
	}
	return nil
}
