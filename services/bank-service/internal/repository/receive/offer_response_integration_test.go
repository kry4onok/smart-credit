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

const insertOfferCaseTestQuery = `
INSERT INTO cases (id, org_id, client_user_id, product_id, product_version, product_name,
    company_inn, company_name, company_snapshot, application_snapshot, documents_snapshot,
    requested_amount_kopecks, requested_term_months, purpose_tag, status,
    client_event_version, version, audit_version)
VALUES ($1, $2, $3, $4, 1, 'Test', '7707083893', 'Test', '{}'::jsonb,
    '{}'::jsonb, '[]'::jsonb, 1000, 12, 'WORKING_CAPITAL', $5, 2, 5, 1)`

const insertOfferDecisionTestQuery = `
INSERT INTO decisions (id, case_id, actor_type, outcome)
VALUES ($1, $2, 'SYSTEM', 'APPROVE')`

const insertOfferTestQuery = `
INSERT INTO offers (id, case_id, decision_id, kind, product_id, product_version,
    product_name, amount_kopecks, term_months, rate_bps, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, 1, 'Test', 1000, 12, 1000, $6, $7)`

const selectOfferResponseStateTestQuery = `
SELECT status, client_event_version, version, audit_version FROM cases WHERE id = $1`

const selectOfferStatusTestQuery = `SELECT status, accepted_at FROM offers WHERE id = $1`

const selectOfferAuditTestQuery = `
SELECT action, actor_type, actor_id, case_version, details->>'event_id', details->>'offer_id'
FROM audit_log WHERE case_id = $1`

const selectOfferOutboxTestQuery = `
SELECT aggregate_version, payload FROM outbox WHERE aggregate_id = $1`

const savepointOfferTestQuery = `SAVEPOINT invalid_offer`
const rollbackOfferTestQuery = `ROLLBACK TO SAVEPOINT invalid_offer`
const savepointExpiredOfferTestQuery = `SAVEPOINT expired_offer`
const rollbackExpiredOfferTestQuery = `ROLLBACK TO SAVEPOINT expired_offer`
const savepointWrongStatusTestQuery = `SAVEPOINT wrong_status`
const rollbackWrongStatusTestQuery = `ROLLBACK TO SAVEPOINT wrong_status`
const expireOfferTestQuery = `UPDATE offers SET expires_at = created_at + interval '1 second' WHERE id = $1`
const setWrongCaseStatusTestQuery = `UPDATE cases SET status = 'MANUAL_REVIEW' WHERE id = $1`

func TestApplyOfferResponsesAtomically(t *testing.T) {
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
		name, fromStatus, action, toStatus, offerStatus string
		publicStatus                                    eventspb.ClientStatus
	}{
		{"accept", "OFFER", "OfferAccepted", "OFFER_ACCEPTED", "ACCEPTED", eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING},
		{"decline", "COUNTER_OFFER", "OfferDeclined", "CANCELLED", "DECLINED", eventspb.ClientStatus_CLIENT_STATUS_CLOSED},
	} {
		t.Run(test.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			repo := New(tx)
			caseID, userID, productID, decisionID, offerID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
			if _, err := tx.Exec(ctx, insertOfferCaseTestQuery, caseID, uuid.New(), userID, productID, test.fromStatus); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, insertOfferDecisionTestQuery, decisionID, caseID); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, insertOfferTestQuery, offerID, caseID, decisionID, test.fromStatus,
				productID, time.Now().Add(time.Hour), time.Now().Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			input := receive.Input{EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 3,
				OfferAction: test.action, OfferID: offerID}
			conflict := func(name string, invalid receive.Input) {
				t.Helper()
				if _, err := tx.Exec(ctx, savepointOfferTestQuery); err != nil {
					t.Fatal(err)
				}
				invalid.EventID = uuid.New()
				if applied, err := repo.Apply(ctx, invalid); applied || !errors.Is(err, ErrOfferResponseConflict) {
					t.Fatalf("%s = %v, %v", name, applied, err)
				}
				if _, err := tx.Exec(ctx, rollbackOfferTestQuery); err != nil {
					t.Fatal(err)
				}
			}
			invalid := input
			invalid.OfferID = uuid.New()
			conflict("wrong offer", invalid)
			invalid = input
			invalid.AggregateVersion = 2
			conflict("stale version", invalid)
			invalid = input
			invalid.AggregateVersion = 4
			conflict("version gap", invalid)
			if _, err := tx.Exec(ctx, savepointExpiredOfferTestQuery); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, expireOfferTestQuery, offerID); err != nil {
				t.Fatal(err)
			}
			if applied, err := repo.Apply(ctx, input); err != nil || !applied {
				t.Fatalf("expired response = %v, %v", applied, err)
			}
			var expiredCase, expiredOffer string
			var expiredVersion, expiredClientVersion, expiredAuditVersion int64
			if err := tx.QueryRow(ctx, selectOfferResponseStateTestQuery, caseID).Scan(
				&expiredCase, &expiredClientVersion, &expiredVersion, &expiredAuditVersion); err != nil {
				t.Fatal(err)
			}
			var expiredAcceptedAt *time.Time
			if err := tx.QueryRow(ctx, selectOfferStatusTestQuery, offerID).Scan(&expiredOffer, &expiredAcceptedAt); err != nil {
				t.Fatal(err)
			}
			if expiredCase != "EXPIRED" || expiredOffer != "EXPIRED" || expiredAcceptedAt != nil ||
				expiredVersion != 6 || expiredClientVersion != 3 || expiredAuditVersion != 2 {
				t.Fatalf("expired case/offer = %s/%s, versions %d/%d/%d", expiredCase, expiredOffer,
					expiredVersion, expiredClientVersion, expiredAuditVersion)
			}
			var expiredOutboxVersion int64
			var expiredPayload []byte
			if err := tx.QueryRow(ctx, selectOfferOutboxTestQuery, caseID.String()).Scan(&expiredOutboxVersion, &expiredPayload); err != nil {
				t.Fatal(err)
			}
			var expiredEvent eventspb.Event
			if err := proto.Unmarshal(expiredPayload, &expiredEvent); err != nil {
				t.Fatal(err)
			}
			if expiredOutboxVersion != 6 || expiredEvent.GetApplicationPublicStatePublished().GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_CLOSED {
				t.Fatalf("expired public event = %v", &expiredEvent)
			}
			if _, err := tx.Exec(ctx, rollbackExpiredOfferTestQuery); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, savepointWrongStatusTestQuery); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, setWrongCaseStatusTestQuery, caseID); err != nil {
				t.Fatal(err)
			}
			conflict("wrong status", input)
			if _, err := tx.Exec(ctx, rollbackWrongStatusTestQuery); err != nil {
				t.Fatal(err)
			}
			if applied, err := repo.Apply(ctx, input); err != nil || !applied {
				t.Fatalf("apply response = %v, %v", applied, err)
			}
			if applied, err := repo.Apply(ctx, input); err != nil || applied {
				t.Fatalf("replay = %v, %v", applied, err)
			}
			var status string
			var clientVersion, caseVersion, auditVersion int64
			if err := tx.QueryRow(ctx, selectOfferResponseStateTestQuery, caseID).Scan(&status, &clientVersion, &caseVersion, &auditVersion); err != nil {
				t.Fatal(err)
			}
			if status != test.toStatus || clientVersion != 3 || caseVersion != 6 || auditVersion != 2 {
				t.Fatalf("case = %s, versions %d/%d/%d", status, clientVersion, caseVersion, auditVersion)
			}
			var offerStatus string
			var acceptedAt *time.Time
			if err := tx.QueryRow(ctx, selectOfferStatusTestQuery, offerID).Scan(&offerStatus, &acceptedAt); err != nil {
				t.Fatal(err)
			}
			if offerStatus != test.offerStatus || (acceptedAt != nil) != (test.action == "OfferAccepted") {
				t.Fatalf("offer = %s, accepted_at = %v", offerStatus, acceptedAt)
			}
			var action, actorType, storedEventID, storedOfferID string
			var actorID uuid.UUID
			var auditCaseVersion int64
			if err := tx.QueryRow(ctx, selectOfferAuditTestQuery, caseID).Scan(&action, &actorType, &actorID,
				&auditCaseVersion, &storedEventID, &storedOfferID); err != nil {
				t.Fatal(err)
			}
			if action != test.action || actorType != "CLIENT" || actorID != userID || auditCaseVersion != 6 ||
				storedEventID != input.EventID.String() || storedOfferID != offerID.String() {
				t.Fatalf("audit = %s/%s/%s/%d/%s/%s", action, actorType, actorID, auditCaseVersion, storedEventID, storedOfferID)
			}
			var outboxVersion int64
			var payload []byte
			if err := tx.QueryRow(ctx, selectOfferOutboxTestQuery, caseID.String()).Scan(&outboxVersion, &payload); err != nil {
				t.Fatal(err)
			}
			var published eventspb.Event
			if err := proto.Unmarshal(payload, &published); err != nil {
				t.Fatal(err)
			}
			if outboxVersion != 6 || published.GetAggregateVersion() != 6 ||
				published.GetApplicationPublicStatePublished().GetStatus() != test.publicStatus ||
				published.GetApplicationPublicStatePublished().GetActiveOffer() != nil {
				t.Fatalf("public event = %v", &published)
			}
			var processed int
			if err := tx.QueryRow(ctx, countProcessedEventQuery, input.EventID).Scan(&processed); err != nil {
				t.Fatal(err)
			}
			if processed != 1 {
				t.Fatalf("processed events = %d", processed)
			}
		})
	}
}
