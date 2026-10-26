package receiverepository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSkipStaleClientEventKeepsCaseAndAdvancesClientVersion(t *testing.T) {
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
	repo := New(tx)
	caseID := uuid.New()
	if _, err := tx.Exec(ctx, insertOfferCaseTestQuery, caseID, uuid.New(), uuid.New(), uuid.New(), "REJECTED"); err != nil {
		t.Fatal(err)
	}
	cancel := receive.Input{Cancelled: true, CancelReason: "Не требуется", EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 3}
	if _, err := tx.Exec(ctx, savepointCancellationTestQuery); err != nil {
		t.Fatal(err)
	}
	if applied, err := repo.Apply(ctx, cancel); applied || !errors.Is(err, receive.ErrStaleEvent) {
		t.Fatalf("cancellation of a rejected case = %v, %v", applied, err)
	}
	if _, err := tx.Exec(ctx, rollbackCancellationTestQuery); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := repo.Skip(ctx, cancel, ErrCancellationConflict.Error()); err != nil {
			t.Fatalf("skip = %v", err)
		}
	}
	if applied, err := repo.Apply(ctx, cancel); applied || err != nil {
		t.Fatalf("redelivery after skip = %v, %v", applied, err)
	}
	// The next client event still lines up with the +1 version check.
	next := cancel
	next.EventID, next.AggregateVersion = uuid.New(), 4
	if _, err := tx.Exec(ctx, savepointCancellationTestQuery); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Apply(ctx, next); !errors.Is(err, receive.ErrStaleEvent) {
		t.Fatalf("next stale event = %v", err)
	}
	if _, err := tx.Exec(ctx, rollbackCancellationTestQuery); err != nil {
		t.Fatal(err)
	}
	if err := repo.Skip(ctx, next, ErrCancellationConflict.Error()); err != nil {
		t.Fatal(err)
	}

	var status string
	var clientVersion, caseVersion int64
	if err := tx.QueryRow(ctx, `SELECT status, client_event_version, version FROM cases WHERE id = $1`, caseID).
		Scan(&status, &clientVersion, &caseVersion); err != nil {
		t.Fatal(err)
	}
	if status != "REJECTED" || clientVersion != 4 || caseVersion != 5 {
		t.Fatalf("case after skips: status=%s client_event_version=%d version=%d", status, clientVersion, caseVersion)
	}
	var skipped, processed int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE case_id = $1 AND action = 'CLIENT_EVENT_SKIPPED'`, caseID).Scan(&skipped); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE aggregate_id = $1`, caseID).Scan(&processed); err != nil {
		t.Fatal(err)
	}
	if skipped != 2 || processed != 2 {
		t.Fatalf("skipped audits = %d, processed events = %d", skipped, processed)
	}
	var outbox int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE aggregate_id = $1`, caseID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if outbox != 0 {
		t.Fatalf("skipped events published %d public states", outbox)
	}
}
