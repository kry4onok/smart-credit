package receiverepository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	disbursementrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/disbursement"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const acceptOfferForSigningTestQuery = `UPDATE offers SET status = 'ACCEPTED', accepted_at = $2 WHERE id = $1`
const selectSigningStateTestQuery = `SELECT status, client_event_version, version, audit_version, next_run_at FROM cases WHERE id = $1`
const selectSignedOfferTestQuery = `SELECT status, signed_at FROM offers WHERE id = $1`
const selectSigningAuditTestQuery = `SELECT action, actor_type, actor_id, case_version, reason FROM audit_log WHERE case_id = $1 ORDER BY case_version`
const selectSigningOutboxTestQuery = `SELECT aggregate_version, payload FROM outbox WHERE aggregate_id = $1 ORDER BY aggregate_version`
const makeDisbursementDueTestQuery = `UPDATE cases SET next_run_at = now() - interval '100 years' WHERE id = $1`
const savepointSigningTestQuery = `SAVEPOINT invalid_signing`
const rollbackSigningTestQuery = `ROLLBACK TO SAVEPOINT invalid_signing`

func TestContractSignedAndDemoDisbursement(t *testing.T) {
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
	caseID, userID, productID, decisionID, offerID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, insertOfferCaseTestQuery, caseID, uuid.New(), userID, productID, "OFFER_ACCEPTED"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, insertOfferDecisionTestQuery, decisionID, caseID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, insertOfferTestQuery, offerID, caseID, decisionID,
		"OFFER", productID, time.Now().Add(time.Hour), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	acceptedAt := time.Now().Add(-time.Minute)
	if _, err := tx.Exec(ctx, acceptOfferForSigningTestQuery, offerID, acceptedAt); err != nil {
		t.Fatal(err)
	}
	signedAt := time.Now().UTC()
	input := receive.Input{EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 3,
		OfferAction: "ContractSigned", OfferID: offerID, SignedAt: signedAt}
	conflict := func(name string, invalid receive.Input) {
		t.Helper()
		if _, err := tx.Exec(ctx, savepointSigningTestQuery); err != nil {
			t.Fatal(err)
		}
		invalid.EventID = uuid.New()
		if applied, err := repo.Apply(ctx, invalid); applied || !errors.Is(err, ErrSigningConflict) {
			t.Fatalf("%s = %v, %v", name, applied, err)
		}
		if _, err := tx.Exec(ctx, rollbackSigningTestQuery); err != nil {
			t.Fatal(err)
		}
	}
	wrong := input
	wrong.OfferID = uuid.New()
	conflict("wrong offer", wrong)
	wrong = input
	wrong.AggregateVersion = 4
	conflict("version gap", wrong)
	wrong = input
	wrong.SignedAt = acceptedAt.Add(-time.Second)
	conflict("signed before acceptance", wrong)
	wrong = input
	wrong.SignedAt = time.Now().Add(6 * time.Minute)
	conflict("future signature", wrong)
	if applied, err := repo.Apply(ctx, input); err != nil || !applied {
		t.Fatalf("sign = %v, %v", applied, err)
	}
	if applied, err := repo.Apply(ctx, input); err != nil || applied {
		t.Fatalf("replay = %v, %v", applied, err)
	}
	var status string
	var clientVersion, caseVersion, auditVersion int64
	var due *time.Time
	if err := tx.QueryRow(ctx, selectSigningStateTestQuery, caseID).Scan(&status, &clientVersion, &caseVersion, &auditVersion, &due); err != nil {
		t.Fatal(err)
	}
	if status != "SIGNING" || clientVersion != 3 || caseVersion != 6 || auditVersion != 2 || due == nil || due.Before(signedAt.Add(2*time.Second)) {
		t.Fatalf("signing state = %s, versions %d/%d/%d, due %v", status, clientVersion, caseVersion, auditVersion, due)
	}
	var offerStatus string
	var storedSignedAt *time.Time
	if err := tx.QueryRow(ctx, selectSignedOfferTestQuery, offerID).Scan(&offerStatus, &storedSignedAt); err != nil {
		t.Fatal(err)
	}
	if offerStatus != "SIGNED" || storedSignedAt == nil || !storedSignedAt.Equal(signedAt) {
		t.Fatalf("signed offer = %s at %v", offerStatus, storedSignedAt)
	}
	if _, err := tx.Exec(ctx, savepointSigningTestQuery); err != nil {
		t.Fatal(err)
	}
	duplicateVersion := input
	duplicateVersion.EventID = uuid.New()
	if applied, err := repo.Apply(ctx, duplicateVersion); applied || !errors.Is(err, ErrEventConflict) {
		t.Fatalf("different event at signed version = %v, %v", applied, err)
	}
	if _, err := tx.Exec(ctx, rollbackSigningTestQuery); err != nil {
		t.Fatal(err)
	}
	alreadySigned := input
	alreadySigned.AggregateVersion = 4
	conflict("already signed", alreadySigned)
	cancel := receive.Input{EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 4, Cancelled: true}
	if _, err := tx.Exec(ctx, savepointSigningTestQuery); err != nil {
		t.Fatal(err)
	}
	if applied, err := repo.Apply(ctx, cancel); applied || !errors.Is(err, ErrCancellationConflict) {
		t.Fatalf("cancel after signing = %v, %v", applied, err)
	}
	if _, err := tx.Exec(ctx, rollbackSigningTestQuery); err != nil {
		t.Fatal(err)
	}
	worker := disbursementrepository.New(tx, transaction.New(tx))
	if _, err := tx.Exec(ctx, makeDisbursementDueTestQuery, caseID); err != nil {
		t.Fatal(err)
	}
	if completed, err := worker.RunOnce(ctx); err != nil || !completed {
		t.Fatalf("due disbursement = %v, %v", completed, err)
	}
	if err := tx.QueryRow(ctx, selectSigningStateTestQuery, caseID).Scan(&status, &clientVersion, &caseVersion, &auditVersion, &due); err != nil {
		t.Fatal(err)
	}
	if status != "DISBURSED" || clientVersion != 3 || caseVersion != 7 || auditVersion != 3 || due != nil {
		t.Fatalf("disbursed state = %s, versions %d/%d/%d, due %v", status, clientVersion, caseVersion, auditVersion, due)
	}
	auditRows, err := tx.Query(ctx, selectSigningAuditTestQuery, caseID)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for auditRows.Next() {
		var action, actorType, reason string
		var actorID *uuid.UUID
		var version int64
		if err := auditRows.Scan(&action, &actorType, &actorID, &version, &reason); err != nil {
			t.Fatal(err)
		}
		if action == "CONTRACT_SIGNED" && (actorType != "CLIENT" || actorID == nil || *actorID != userID || version != 6 || reason == "") {
			t.Fatalf("signing audit = %s/%s/%v/%d", action, actorType, actorID, version)
		}
		if action == "DEMO_DISBURSED" && (actorType != "SYSTEM" || actorID != nil || version != 7 || reason == "") {
			t.Fatalf("disbursement audit = %s/%s/%v/%d", action, actorType, actorID, version)
		}
		actions = append(actions, action)
	}
	if err := auditRows.Err(); err != nil {
		t.Fatal(err)
	}
	auditRows.Close()
	if len(actions) != 2 || actions[0] != "CONTRACT_SIGNED" || actions[1] != "DEMO_DISBURSED" {
		t.Fatalf("audit actions = %v", actions)
	}
	outboxRows, err := tx.Query(ctx, selectSigningOutboxTestQuery, caseID.String())
	if err != nil {
		t.Fatal(err)
	}
	var events []eventspb.ClientStatus
	for outboxRows.Next() {
		var version int64
		var payload []byte
		if err := outboxRows.Scan(&version, &payload); err != nil {
			t.Fatal(err)
		}
		var event eventspb.Event
		if err := proto.Unmarshal(payload, &event); err != nil {
			t.Fatal(err)
		}
		if version != int64(6+len(events)) || event.GetAggregateVersion() != version {
			t.Fatalf("outbox version = %d/%d", version, event.GetAggregateVersion())
		}
		events = append(events, event.GetApplicationPublicStatePublished().GetStatus())
	}
	if err := outboxRows.Err(); err != nil {
		t.Fatal(err)
	}
	outboxRows.Close()
	if len(events) != 2 || events[0] != eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING ||
		events[1] != eventspb.ClientStatus_CLIENT_STATUS_DISBURSED {
		t.Fatalf("public events = %v", events)
	}
}
