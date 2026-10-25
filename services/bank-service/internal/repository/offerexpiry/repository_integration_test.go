package offerexpiryrepository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	receiverepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/receive"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const insertCaseTestQuery = `
INSERT INTO cases (id, org_id, client_user_id, product_id, product_version, product_name,
    company_inn, company_name, company_snapshot, application_snapshot, documents_snapshot,
    requested_amount_kopecks, requested_term_months, purpose_tag, status,
    client_event_version, version, audit_version)
VALUES ($1, $2, $3, $4, 1, 'Test', '7707083893', 'Test', '{}'::jsonb,
    '{}'::jsonb, '[]'::jsonb, 1000, 12, 'WORKING_CAPITAL', $5, 2, 5, 1)`

const insertDecisionTestQuery = `
INSERT INTO decisions (id, case_id, actor_type, outcome)
VALUES ($1, $2, 'SYSTEM', 'APPROVE')`

const insertOfferTestQuery = `
INSERT INTO offers (id, case_id, decision_id, kind, product_id, product_version,
    product_name, amount_kopecks, term_months, rate_bps, expires_at, created_at, status)
VALUES ($1, $2, $3, $4, $5, 1, 'Test', 1000, 12, 1000, $6, $7, $8)`

const selectCaseTestQuery = `SELECT status, client_event_version, version, audit_version FROM cases WHERE id = $1`
const selectOfferTestQuery = `SELECT status FROM offers WHERE id = $1`
const selectAuditTestQuery = `SELECT action, actor_type, case_version, details->>'offer_id' FROM audit_log WHERE case_id = $1`
const selectOutboxTestQuery = `SELECT aggregate_version, payload FROM outbox WHERE aggregate_id = $1`
const countOutboxTestQuery = `SELECT count(*) FROM outbox WHERE aggregate_id = $1`
const savepointLateResponseTestQuery = `SAVEPOINT late_response`
const rollbackLateResponseTestQuery = `ROLLBACK TO SAVEPOINT late_response`
const deleteOutboxTestQuery = `DELETE FROM outbox WHERE aggregate_id = $1`
const deleteAuditTestQuery = `DELETE FROM audit_log WHERE case_id = $1`
const deleteProcessedTestQuery = `DELETE FROM processed_events WHERE aggregate_id = $1`
const deleteOfferTestQuery = `DELETE FROM offers WHERE case_id = $1`
const deleteDecisionTestQuery = `DELETE FROM decisions WHERE case_id = $1`
const deleteCaseTestQuery = `DELETE FROM cases WHERE id = $1`

func TestExpiresAvailableOffersAtomicallyAndOnlyOnce(t *testing.T) {
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
	repo := New(tx, transaction.New(tx))
	caseID, offerID := uuid.New(), uuid.New()
	productID, decisionID := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, insertCaseTestQuery, caseID, uuid.New(), uuid.New(), productID, "COUNTER_OFFER"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, insertDecisionTestQuery, decisionID, caseID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, insertOfferTestQuery, offerID, caseID, decisionID, "COUNTER_OFFER", productID,
		time.Now().Add(-time.Minute), time.Now().Add(-time.Hour), "AVAILABLE"); err != nil {
		t.Fatal(err)
	}
	if expired, err := repo.RunOnce(ctx); err != nil || !expired {
		t.Fatalf("expire = %v, %v", expired, err)
	}
	if expired, err := repo.RunOnce(ctx); err != nil || expired {
		t.Fatalf("repeat = %v, %v", expired, err)
	}
	var status string
	var clientVersion, version, auditVersion int64
	if err := tx.QueryRow(ctx, selectCaseTestQuery, caseID).Scan(&status, &clientVersion, &version, &auditVersion); err != nil {
		t.Fatal(err)
	}
	if status != "EXPIRED" || clientVersion != 2 || version != 6 || auditVersion != 2 {
		t.Fatalf("case = %s, versions %d/%d/%d", status, clientVersion, version, auditVersion)
	}
	if err := tx.QueryRow(ctx, selectOfferTestQuery, offerID).Scan(&status); err != nil || status != "EXPIRED" {
		t.Fatalf("offer = %s, %v", status, err)
	}
	var action, actorType, storedOfferID string
	var auditCaseVersion int64
	if err := tx.QueryRow(ctx, selectAuditTestQuery, caseID).Scan(&action, &actorType, &auditCaseVersion, &storedOfferID); err != nil {
		t.Fatal(err)
	}
	if action != "OFFER_EXPIRED" || actorType != "SYSTEM" || auditCaseVersion != 6 || storedOfferID != offerID.String() {
		t.Fatalf("audit = %s/%s/%d/%s", action, actorType, auditCaseVersion, storedOfferID)
	}
	var outboxVersion int64
	var payload []byte
	if err := tx.QueryRow(ctx, selectOutboxTestQuery, caseID.String()).Scan(&outboxVersion, &payload); err != nil {
		t.Fatal(err)
	}
	var event eventspb.Event
	if err := proto.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	if outboxVersion != 6 || event.GetAggregateVersion() != 6 ||
		event.GetApplicationPublicStatePublished().GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_CLOSED {
		t.Fatalf("public state = %v", &event)
	}
	var outboxCount int
	if err := tx.QueryRow(ctx, countOutboxTestQuery, caseID.String()).Scan(&outboxCount); err != nil || outboxCount != 1 {
		t.Fatalf("outbox count = %d, %v", outboxCount, err)
	}
	for _, late := range []struct {
		name, action string
		cancel       bool
		wantErr      error
	}{
		{"accept", "OfferAccepted", false, receiverepository.ErrOfferResponseConflict},
		{"decline", "OfferDeclined", false, receiverepository.ErrOfferResponseConflict},
		{"cancel", "", true, receiverepository.ErrCancellationConflict},
	} {
		if _, err := tx.Exec(ctx, savepointLateResponseTestQuery); err != nil {
			t.Fatal(err)
		}
		input := receive.Input{EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 3,
			OfferAction: late.action, OfferID: offerID, Cancelled: late.cancel}
		if applied, err := receiverepository.New(tx).Apply(ctx, input); applied || !errors.Is(err, late.wantErr) {
			t.Fatalf("late %s = %v, %v", late.name, applied, err)
		}
		if _, err := tx.Exec(ctx, rollbackLateResponseTestQuery); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClientResponsesWinBeforeExpiryWorker(t *testing.T) {
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
	for _, test := range []struct {
		name, action, caseStatus, offerStatus string
		cancel                                bool
	}{
		{"accept", "OfferAccepted", "OFFER_ACCEPTED", "ACCEPTED", false},
		{"decline", "OfferDeclined", "CANCELLED", "DECLINED", false},
		{"cancel", "", "CANCELLED", "DECLINED", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			caseID, offerID := uuid.New(), uuid.New()
			productID, decisionID := uuid.New(), uuid.New()
			if _, err := tx.Exec(ctx, insertCaseTestQuery, caseID, uuid.New(), uuid.New(), productID, "OFFER"); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, insertDecisionTestQuery, decisionID, caseID); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, insertOfferTestQuery, offerID, caseID, decisionID, "OFFER", productID,
				time.Now().Add(time.Hour), time.Now().Add(-time.Hour), "AVAILABLE"); err != nil {
				t.Fatal(err)
			}
			input := receive.Input{EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 3,
				OfferAction: test.action, OfferID: offerID, Cancelled: test.cancel}
			if applied, err := receiverepository.New(tx).Apply(ctx, input); err != nil || !applied {
				t.Fatalf("client action = %v, %v", applied, err)
			}
			var status string
			var clientVersion, version, auditVersion int64
			if err := tx.QueryRow(ctx, selectCaseTestQuery, caseID).Scan(&status, &clientVersion, &version, &auditVersion); err != nil {
				t.Fatal(err)
			}
			if status != test.caseStatus || clientVersion != 3 || version != 6 || auditVersion != 2 {
				t.Fatalf("case = %s, versions %d/%d/%d", status, clientVersion, version, auditVersion)
			}
			if err := tx.QueryRow(ctx, selectOfferTestQuery, offerID).Scan(&status); err != nil || status != test.offerStatus {
				t.Fatalf("offer = %s, %v", status, err)
			}
			if _, err := New(tx, transaction.New(tx)).RunOnce(ctx); err != nil {
				t.Fatalf("expiry after client action: %v", err)
			}
			if err := tx.QueryRow(ctx, selectCaseTestQuery, caseID).Scan(&status, &clientVersion, &version, &auditVersion); err != nil {
				t.Fatal(err)
			}
			if status != test.caseStatus || clientVersion != 3 || version != 6 || auditVersion != 2 {
				t.Fatalf("case after worker = %s, versions %d/%d/%d", status, clientVersion, version, auditVersion)
			}
			var count int
			if err := tx.QueryRow(ctx, countOutboxTestQuery, caseID.String()).Scan(&count); err != nil || count != 1 {
				t.Fatalf("outbox count = %d, %v", count, err)
			}
		})
	}
}

func TestExpiryWorkerSkipsCaseLockedByClientResponse(t *testing.T) {
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
	for _, test := range []struct {
		name, action, wantStatus string
		cancel                   bool
	}{
		{"accept", "OfferAccepted", "EXPIRED", false},
		{"decline", "OfferDeclined", "EXPIRED", false},
		{"cancel", "", "CANCELLED", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseID, offerID := uuid.New(), uuid.New()
			productID, decisionID := uuid.New(), uuid.New()
			t.Cleanup(func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				for _, query := range []string{deleteOutboxTestQuery, deleteAuditTestQuery,
					deleteProcessedTestQuery, deleteOfferTestQuery, deleteDecisionTestQuery, deleteCaseTestQuery} {
					if _, err := pool.Exec(cleanupCtx, query, caseID); err != nil {
						t.Errorf("cleanup case %s: %v", caseID, err)
					}
				}
			})
			if _, err := pool.Exec(ctx, insertCaseTestQuery, caseID, uuid.New(), uuid.New(), productID, "OFFER"); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, insertDecisionTestQuery, decisionID, caseID); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, insertOfferTestQuery, offerID, caseID, decisionID, "OFFER", productID,
				time.Now().Add(-time.Minute), time.Now().Add(-time.Hour), "AVAILABLE"); err != nil {
				t.Fatal(err)
			}
			responseTx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = responseTx.Rollback(ctx) }()
			input := receive.Input{EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 3,
				OfferAction: test.action, OfferID: offerID, Cancelled: test.cancel}
			if applied, err := receiverepository.New(responseTx).Apply(ctx, input); err != nil || !applied {
				t.Fatalf("client action = %v, %v", applied, err)
			}
			workerCtx, cancelWorker := context.WithTimeout(ctx, 2*time.Second)
			defer cancelWorker()
			if _, err := New(pool, transaction.New(pool)).RunOnce(workerCtx); err != nil {
				t.Fatalf("worker while case locked: %v", err)
			}
			if err := responseTx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := New(pool, transaction.New(pool)).RunOnce(ctx); err != nil {
				t.Fatal(err)
			}
			var status string
			var clientVersion, version, auditVersion int64
			if err := pool.QueryRow(ctx, selectCaseTestQuery, caseID).Scan(&status, &clientVersion, &version, &auditVersion); err != nil {
				t.Fatal(err)
			}
			if status != test.wantStatus || clientVersion != 3 || version != 6 || auditVersion != 2 {
				t.Fatalf("case = %s, versions %d/%d/%d", status, clientVersion, version, auditVersion)
			}
		})
	}
}
