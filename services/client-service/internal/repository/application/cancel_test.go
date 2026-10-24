package application

import (
	"crypto/sha256"
	"errors"
	"slices"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func cancellationParams(f applicationFixture) model.CancelApplicationParams {
	hash := sha256.Sum256([]byte("cancel"))
	return model.CancelApplicationParams{
		OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
		IdempotencyID: uuid.New(), Reason: "Финансирование больше не требуется", RequestHash: hash[:],
	}
}

func TestCancelApplicationIntegration(t *testing.T) {
	for _, status := range []eventspb.ClientStatus{
		eventspb.ClientStatus_CLIENT_STATUS_SUBMITTED,
		eventspb.ClientStatus_CLIENT_STATUS_UNDER_REVIEW,
		eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO,
		eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE,
		eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING,
	} {
		t.Run(status.String(), func(t *testing.T) {
			f := newApplicationFixture(t)
			f.submit(t)
			f.project(t, 1, &eventspb.ApplicationPublicStatePublished{Status: status, StageLabel: "Текущий этап"})
			before, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || !slices.Contains(before.AllowedActions, "CANCEL") {
				t.Fatalf("before cancel=%+v, err=%v", before, err)
			}
			params := cancellationParams(f)
			first, err := f.repo.CancelApplication(t.Context(), params)
			if err != nil || first.PendingAction != "CANCEL_PENDING" || first.ApplicationID != f.draft.ID {
				t.Fatalf("accepted=%+v, err=%v", first, err)
			}
			got, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || got.Status != before.Status || got.PendingAction == nil || *got.PendingAction != "CANCEL_PENDING" || got.Version != before.Version+1 || len(got.AllowedActions) != 0 {
				t.Fatalf("after cancel=%+v, err=%v", got, err)
			}
			event := f.event(t, "ApplicationCancelled")
			if event.GetAggregateVersion() != 2 || event.GetAggregateId() != f.draft.ID.String() || event.GetAggregateType() != "APPLICATION" || event.GetApplicationCancelled().GetReason() != params.Reason || event.GetOccurredAt().CheckValid() != nil || event.GetCorrelationId() == "" {
				t.Fatalf("cancel event=%v", event)
			}
			var actorID uuid.UUID
			var eventID, timelineStatus string
			if err := f.db.QueryRow(t.Context(), `SELECT actor_id, event_id::text, status FROM timeline_entries WHERE application_id = $1 AND action = 'CANCEL'`, f.draft.ID).Scan(&actorID, &eventID, &timelineStatus); err != nil || actorID != params.UserID || eventID != event.GetEventId() || timelineStatus != before.Status {
				t.Fatalf("timeline actor=%s event=%s status=%s err=%v", actorID, eventID, timelineStatus, err)
			}
			// A retry must replay even after the bank has confirmed closure.
			f.project(t, 2, &eventspb.ApplicationPublicStatePublished{Status: eventspb.ClientStatus_CLIENT_STATUS_CLOSED, StageLabel: "Закрыта"})
			if replay, err := f.repo.CancelApplication(t.Context(), params); err != nil || replay != first {
				t.Fatalf("replay=%+v, err=%v", replay, err)
			}
			params.RequestHash = make([]byte, 32)
			if _, err := f.repo.CancelApplication(t.Context(), params); !errors.Is(err, domain.ErrIdempotencyConflict) {
				t.Fatalf("changed body: err=%v", err)
			}
			params.IdempotencyID = uuid.New()
			if _, err := f.repo.CancelApplication(t.Context(), params); !errors.Is(err, domain.ErrApplicationCancelConflict) {
				t.Fatalf("new cancellation after closure: err=%v", err)
			}
			var outboxCount, clientVersion, keyCount int
			if err := f.db.QueryRow(t.Context(), `SELECT client_event_version,
			    (SELECT count(*) FROM outbox WHERE event_type = 'ApplicationCancelled'),
			    (SELECT count(*) FROM idempotency_keys WHERE route LIKE '%/cancel' AND application_id = $1)
			    FROM applications WHERE id = $1`, f.draft.ID).Scan(&clientVersion, &outboxCount, &keyCount); err != nil || clientVersion != 2 || outboxCount != 1 || keyCount != 1 {
				t.Fatalf("version=%d outbox=%d keys=%d err=%v", clientVersion, outboxCount, keyCount, err)
			}
		})
	}
}

func TestCancelRejectsStateAndPendingConflictsIntegration(t *testing.T) {
	for _, tc := range []struct {
		status  string
		pending string
	}{
		{"DRAFT", ""}, {"DISBURSED", ""}, {"REJECTED", ""}, {"CLOSED", ""},
		{"SUBMITTED", "INFO_SENT"}, {"SUBMITTED", "ACCEPT_PENDING"},
		{"SUBMITTED", "DECLINE_PENDING"}, {"SUBMITTED", "SIGN_PENDING"}, {"SUBMITTED", "CANCEL_PENDING"},
	} {
		t.Run(tc.status+"/"+tc.pending, func(t *testing.T) {
			f := newApplicationFixture(t)
			if tc.status != "DRAFT" {
				f.submit(t)
				if _, err := f.db.Exec(t.Context(), `UPDATE applications SET status = $2, pending_action = NULLIF($3, ''),
				    pending_since = CASE WHEN $3 = '' THEN NULL ELSE now() END WHERE id = $1`, f.draft.ID, tc.status, tc.pending); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.repo.CancelApplication(t.Context(), cancellationParams(f)); !errors.Is(err, domain.ErrApplicationCancelConflict) {
				t.Fatalf("cancel err=%v", err)
			}
			got, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || slices.Contains(got.AllowedActions, "CANCEL") {
				t.Fatalf("application=%+v err=%v", got, err)
			}
			var commands int
			if err := f.db.QueryRow(t.Context(), `SELECT count(*) FROM outbox WHERE event_type = 'ApplicationCancelled'`).Scan(&commands); err != nil || commands != 0 {
				t.Fatalf("unexpected cancel events=%d err=%v", commands, err)
			}
		})
	}
}

func formalizingApplication(t *testing.T) (applicationFixture, uuid.UUID) {
	t.Helper()
	f := newApplicationFixture(t)
	f.submit(t)
	offerID := uuid.New()
	f.project(t, 1, &eventspb.ApplicationPublicStatePublished{
		Status: eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING, StageLabel: "Оформление",
		ActiveOffer: &commonpb.Offer{
			OfferId: offerID.String(), Kind: commonpb.OfferKind_OFFER_KIND_OFFER,
			ProductId: f.params.ProductID.String(), ProductVersion: 1, ProductName: "Демо-продукт",
			Amount: &commonpb.Money{AmountKopecks: 10000, Currency: "RUB"}, TermMonths: 12,
			RateBps: 1900, ExpiresAt: timestamppb.New(time.Now().Add(72 * time.Hour)),
		},
	})
	return f, offerID
}

func TestCancelRejectsAcknowledgedSignatureIntegration(t *testing.T) {
	f, offerID := formalizingApplication(t)
	hash := sha256.Sum256([]byte("sign"))
	if _, err := f.repo.SignContract(t.Context(), model.SignContractParams{
		OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
		OfferID: offerID, IdempotencyID: uuid.New(), RequestHash: hash[:],
	}); err != nil {
		t.Fatal(err)
	}
	f.project(t, 2, &eventspb.ApplicationPublicStatePublished{Status: eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING, StageLabel: "Договор подписан"})
	got, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || got.PendingAction != nil || len(got.AllowedActions) != 0 {
		t.Fatalf("signed application=%+v err=%v", got, err)
	}
	if _, err := f.repo.CancelApplication(t.Context(), cancellationParams(f)); !errors.Is(err, domain.ErrApplicationCancelConflict) {
		t.Fatalf("cancel after signature err=%v", err)
	}
}

func TestConcurrentSignAndCancellationIntegration(t *testing.T) {
	f, offerID := formalizingApplication(t)
	hash := sha256.Sum256([]byte("sign"))
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, err := f.repo.SignContract(t.Context(), model.SignContractParams{
			OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
			OfferID: offerID, IdempotencyID: uuid.New(), RequestHash: hash[:],
		})
		results <- err
	}()
	go func() {
		<-start
		_, err := f.repo.CancelApplication(t.Context(), cancellationParams(f))
		results <- err
	}()
	close(start)
	succeeded := 0
	for range 2 {
		err := <-results
		if err == nil {
			succeeded++
		} else if !errors.Is(err, domain.ErrApplicationSignConflict) && !errors.Is(err, domain.ErrApplicationCancelConflict) {
			t.Fatalf("unexpected concurrent action error=%v", err)
		}
	}
	var commands int
	if err := f.db.QueryRow(t.Context(), `SELECT count(*) FROM outbox WHERE event_type IN ('ContractSigned', 'ApplicationCancelled')`).Scan(&commands); err != nil || succeeded != 1 || commands != 1 {
		t.Fatalf("successes=%d commands=%d err=%v", succeeded, commands, err)
	}
}

func TestCancelRollsBackWhenOutboxFailsIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	before := f.submit(t)
	if _, err := f.db.Exec(t.Context(), `ALTER TABLE outbox ADD CONSTRAINT reject_cancel CHECK (event_type <> 'ApplicationCancelled')`); err != nil {
		t.Fatal(err)
	}
	params := cancellationParams(f)
	if _, err := f.repo.CancelApplication(t.Context(), params); err == nil {
		t.Fatal("expected outbox insert failure")
	}
	got, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || got.PendingAction != nil || got.Version != before.Version {
		t.Fatalf("failed cancellation changed application=%+v err=%v", got, err)
	}
	var writes int
	if err := f.db.QueryRow(t.Context(), `SELECT
	    (SELECT count(*) FROM timeline_entries WHERE action = 'CANCEL') +
	    (SELECT count(*) FROM idempotency_keys WHERE route LIKE '%/cancel')`).Scan(&writes); err != nil || writes != 0 {
		t.Fatalf("failed cancellation left %d writes, err=%v", writes, err)
	}
}
