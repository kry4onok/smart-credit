package receiverepository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const selectCancellationStateTestQuery = `
SELECT status, client_event_version, version, audit_version, next_run_at, lease_until
FROM cases WHERE id = $1`

const selectCancellationAuditTestQuery = `
SELECT action, actor_type, actor_id, case_version, details->>'from_status',
       details->>'to_status', details->>'event_id', details->>'reason'
FROM audit_log WHERE case_id = $1`

const selectCancellationOutboxTestQuery = `
SELECT aggregate_version, payload FROM outbox WHERE aggregate_id = $1`

const selectCancellationInfoStatusTestQuery = `
SELECT status FROM info_requests WHERE case_id = $1`

const selectCancellationOfferStatusTestQuery = `
SELECT status FROM offers WHERE case_id = $1`

const setAcceptedOfferTestQuery = `UPDATE offers SET status = 'ACCEPTED' WHERE case_id = $1`
const setScheduledCaseTestQuery = `UPDATE cases SET next_run_at = now(), lease_until = now() WHERE id = $1`
const savepointCancellationTestQuery = `SAVEPOINT invalid_cancellation`
const rollbackCancellationTestQuery = `ROLLBACK TO SAVEPOINT invalid_cancellation`

func TestApplyCancellationAtomically(t *testing.T) {
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
		status      string
		allowed     bool
		infoRequest bool
		offer       bool
		offerStatus string
	}{
		{status: "VALIDATION", allowed: true},
		{status: "NEEDS_INFO", allowed: true, infoRequest: true},
		{status: "OFFER", allowed: true, offer: true, offerStatus: "DECLINED"},
		{status: "OFFER_ACCEPTED", allowed: true, offer: true, offerStatus: "ACCEPTED"},
		{status: "SIGNING"},
		{status: "DISBURSED"},
		{status: "CANCELLED"},
	} {
		t.Run(test.status, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			repo := New(tx)
			caseID, userID, productID := uuid.New(), uuid.New(), uuid.New()
			if _, err := tx.Exec(ctx, insertOfferCaseTestQuery, caseID, uuid.New(), userID, productID, test.status); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, setScheduledCaseTestQuery, caseID); err != nil {
				t.Fatal(err)
			}
			if test.infoRequest {
				if _, err := tx.Exec(ctx, insertOpenInfoRequestQuery, uuid.New(), caseID); err != nil {
					t.Fatal(err)
				}
			}
			if test.offer {
				decisionID := uuid.New()
				if _, err := tx.Exec(ctx, insertOfferDecisionTestQuery, decisionID, caseID); err != nil {
					t.Fatal(err)
				}
				if _, err := tx.Exec(ctx, insertOfferTestQuery, uuid.New(), caseID, decisionID,
					"OFFER", productID, time.Now().Add(time.Hour), time.Now().Add(-time.Hour)); err != nil {
					t.Fatal(err)
				}
				if test.status == "OFFER_ACCEPTED" {
					if _, err := tx.Exec(ctx, setAcceptedOfferTestQuery, caseID); err != nil {
						t.Fatal(err)
					}
				}
			}
			input := receive.Input{
				Cancelled: true, CancelReason: "Не требуется", EventID: uuid.New(),
				ApplicationID: caseID, AggregateVersion: 3,
			}
			if _, err := tx.Exec(ctx, savepointCancellationTestQuery); err != nil {
				t.Fatal(err)
			}
			wrongVersion := input
			wrongVersion.AggregateVersion = 4
			wrongVersion.EventID = uuid.New()
			if applied, err := repo.Apply(ctx, wrongVersion); applied || !errors.Is(err, ErrCancellationConflict) {
				t.Fatalf("version gap = %v, %v", applied, err)
			}
			if _, err := tx.Exec(ctx, rollbackCancellationTestQuery); err != nil {
				t.Fatal(err)
			}
			applied, err := repo.Apply(ctx, input)
			if !test.allowed {
				if applied || !errors.Is(err, ErrCancellationConflict) {
					t.Fatalf("forbidden cancellation = %v, %v", applied, err)
				}
				return
			}
			if err != nil || !applied {
				t.Fatalf("cancellation = %v, %v", applied, err)
			}
			if applied, err := repo.Apply(ctx, input); err != nil || applied {
				t.Fatalf("replay = %v, %v", applied, err)
			}
			var status string
			var clientVersion, caseVersion, auditVersion int64
			var nextRunAt, leaseUntil *time.Time
			if err := tx.QueryRow(ctx, selectCancellationStateTestQuery, caseID).Scan(
				&status, &clientVersion, &caseVersion, &auditVersion, &nextRunAt, &leaseUntil); err != nil {
				t.Fatal(err)
			}
			if status != "CANCELLED" || clientVersion != 3 || caseVersion != 6 || auditVersion != 2 ||
				nextRunAt != nil || leaseUntil != nil {
				t.Fatalf("case = %s, versions %d/%d/%d, schedule %v/%v", status,
					clientVersion, caseVersion, auditVersion, nextRunAt, leaseUntil)
			}
			if test.infoRequest {
				var requestStatus string
				if err := tx.QueryRow(ctx, selectCancellationInfoStatusTestQuery, caseID).Scan(&requestStatus); err != nil || requestStatus != "CANCELLED" {
					t.Fatalf("info request = %s, %v", requestStatus, err)
				}
			}
			if test.offer {
				var offerStatus string
				if err := tx.QueryRow(ctx, selectCancellationOfferStatusTestQuery, caseID).Scan(&offerStatus); err != nil || offerStatus != test.offerStatus {
					t.Fatalf("offer = %s, %v", offerStatus, err)
				}
			}
			var action, actorType, fromStatus, toStatus, eventID, reason string
			var actorID uuid.UUID
			var auditedVersion int64
			if err := tx.QueryRow(ctx, selectCancellationAuditTestQuery, caseID).Scan(&action, &actorType,
				&actorID, &auditedVersion, &fromStatus, &toStatus, &eventID, &reason); err != nil {
				t.Fatal(err)
			}
			if action != "APPLICATION_CANCELLED" || actorType != "CLIENT" || actorID != userID ||
				auditedVersion != 6 || fromStatus != test.status || toStatus != "CANCELLED" ||
				eventID != input.EventID.String() || reason != input.CancelReason {
				t.Fatalf("audit = %s/%s/%s/%d/%s/%s/%s/%s", action, actorType, actorID,
					auditedVersion, fromStatus, toStatus, eventID, reason)
			}
			var outboxVersion int64
			var payload []byte
			if err := tx.QueryRow(ctx, selectCancellationOutboxTestQuery, caseID.String()).Scan(&outboxVersion, &payload); err != nil {
				t.Fatal(err)
			}
			var event eventspb.Event
			if err := proto.Unmarshal(payload, &event); err != nil {
				t.Fatal(err)
			}
			if outboxVersion != 6 || event.GetAggregateVersion() != 6 ||
				event.GetApplicationPublicStatePublished().GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_CLOSED ||
				event.GetApplicationPublicStatePublished().GetActiveOffer() != nil ||
				event.GetApplicationPublicStatePublished().GetActiveInfoRequest() != nil {
				t.Fatalf("public state = %v", &event)
			}
			var processed int
			if err := tx.QueryRow(ctx, countProcessedEventQuery, input.EventID).Scan(&processed); err != nil || processed != 1 {
				t.Fatalf("processed = %d, %v", processed, err)
			}
		})
	}
}
