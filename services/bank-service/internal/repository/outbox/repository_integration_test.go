package outboxrepository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertTestEventQuery = `
INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload, created_at)
VALUES ($1, $2, $3, 'ApplicationPublicStatePublished', $4, $5)`

const createTestOutboxQuery = `
CREATE TEMP TABLE outbox (LIKE public.outbox INCLUDING ALL) ON COMMIT DROP`

const selectPublishedQuery = `
SELECT published_at IS NOT NULL, lease_until IS NULL, attempts
FROM outbox WHERE event_id = $1`

func TestRelayClaimsInAggregateOrderAndRecoversLease(t *testing.T) {
	databaseURL := os.Getenv("BANK_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("BANK_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, createTestOutboxQuery); err != nil {
		t.Fatal(err)
	}
	repository := New(tx)
	caseID, otherID := uuid.NewString(), uuid.NewString()
	firstID, secondID, otherEventID := uuid.New(), uuid.New(), uuid.New()
	baseTime := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, event := range []struct {
		id        uuid.UUID
		aggregate string
		version   int64
		payload   []byte
		created   time.Time
	}{
		{firstID, caseID, 2, []byte("first"), baseTime},
		{secondID, caseID, 3, []byte("second"), baseTime.Add(time.Second)},
		{otherEventID, otherID, 1, []byte("other"), baseTime.Add(2 * time.Second)},
	} {
		if _, err := tx.Exec(ctx, insertTestEventQuery,
			event.id, event.aggregate, event.version, event.payload, event.created); err != nil {
			t.Fatal(err)
		}
	}

	first, found, err := repository.ClaimNext(ctx)
	if err != nil || !found || first.ID != firstID || first.AggregateID != caseID ||
		first.Topic != "bank.events" || string(first.Payload) != "first" || first.Attempts != 1 {
		t.Fatalf("first claim = %+v, found=%v, err=%v", first, found, err)
	}
	other, found, err := repository.ClaimNext(ctx)
	if err != nil || !found || other.ID != otherEventID {
		t.Fatalf("other aggregate claim = %+v, found=%v, err=%v", other, found, err)
	}
	if marked, err := repository.MarkPublished(ctx, other); err != nil || !marked {
		t.Fatalf("mark other published = %v, %v", marked, err)
	}
	if pending, found, err := repository.ClaimNext(ctx); err != nil || found {
		t.Fatalf("later version claimed before first: %+v, found=%v, err=%v", pending, found, err)
	}
	if err := repository.Release(ctx, first); err != nil {
		t.Fatal(err)
	}
	first, found, err = repository.ClaimNext(ctx)
	if err != nil || !found || first.ID != firstID || first.Attempts != 2 {
		t.Fatalf("reclaimed first event = %+v, found=%v, err=%v", first, found, err)
	}
	if marked, err := repository.MarkPublished(ctx, first); err != nil || !marked {
		t.Fatalf("mark first published = %v, %v", marked, err)
	}
	if marked, err := repository.MarkPublished(ctx, first); err != nil || marked {
		t.Fatalf("duplicate mark = %v, %v", marked, err)
	}
	second, found, err := repository.ClaimNext(ctx)
	if err != nil || !found || second.ID != secondID || second.AggregateVersion != 3 ||
		string(second.Payload) != "second" {
		t.Fatalf("second version claim = %+v, found=%v, err=%v", second, found, err)
	}
	var published, leaseCleared bool
	var attempts int
	if err := tx.QueryRow(ctx, selectPublishedQuery, firstID).Scan(&published, &leaseCleared, &attempts); err != nil {
		t.Fatal(err)
	}
	if !published || !leaseCleared || attempts != 2 {
		t.Fatalf("first event = published=%v, lease cleared=%v, attempts=%d",
			published, leaseCleared, attempts)
	}
}
