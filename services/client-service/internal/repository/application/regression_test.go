package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/consumer/bankevents"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEventEnums(t *testing.T) {
	for purpose, want := range map[string]commonpb.PurposeTag{
		"WORKING_CAPITAL": commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL,
		"EQUIPMENT":       commonpb.PurposeTag_PURPOSE_TAG_EQUIPMENT,
		"EXPANSION":       commonpb.PurposeTag_PURPOSE_TAG_EXPANSION,
	} {
		if got := formProto(model.ApplicationForm{Purpose: purpose}).GetPurpose(); got != want {
			t.Errorf("purpose %s = %v, want %v", purpose, got, want)
		}
	}
	for documentType, want := range map[string]commonpb.DocumentType{
		"BANK_STATEMENT": commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT,
		"BUSINESS_PLAN":  commonpb.DocumentType_DOCUMENT_TYPE_BUSINESS_PLAN,
	} {
		got := documentMetadataProto([]model.ApplicationDocument{{DocumentType: documentType}})
		if got[0].GetDocumentType() != want {
			t.Errorf("document %s = %v, want %v", documentType, got[0].GetDocumentType(), want)
		}
	}
}

type applicationFixture struct {
	db     *pgxpool.Pool
	repo   *Repository
	params model.CreateApplicationParams
	draft  model.Application
}

func newApplicationFixture(t *testing.T) applicationFixture {
	t.Helper()
	db := testutil.Postgres(t)
	params := model.CreateApplicationParams{OrgID: uuid.New(), UserID: uuid.New(), ProductID: uuid.New(), IdempotencyID: uuid.New()}
	if _, err := db.Exec(t.Context(), `
		INSERT INTO companies (org_id, inn, name, okved, registered_at,
		    revenue_12m_kopecks, profit_12m_kopecks, obligations_kopecks)
		VALUES ($1, '7707083893', 'Демо', '62.01', '2020-01-02', 100000000, 10000000, 0)`, params.OrgID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), `
		INSERT INTO products (id, version, code, name, purpose_tags, min_amount_kopecks,
		    max_amount_kopecks, min_term_months, max_term_months, required_document_types)
		VALUES ($1, 1, 'DEMO', 'Демо-продукт', ARRAY['WORKING_CAPITAL'],
		    100, 100000000, 1, 36, ARRAY['BANK_STATEMENT', 'BUSINESS_PLAN'])`, params.ProductID); err != nil {
		t.Fatal(err)
	}
	repo := New(Adapt(db), t.TempDir())
	draft, err := repo.Create(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	return applicationFixture{db: db, repo: repo, params: params, draft: draft}
}

func (f applicationFixture) submit(t *testing.T) model.Application {
	t.Helper()
	for _, documentType := range []string{"BANK_STATEMENT", "BUSINESS_PLAN"} {
		if _, err := f.repo.AddDocument(t.Context(), model.AddApplicationDocumentParams{
			OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
			IdempotencyKey: uuid.New(), DocumentType: documentType, FileName: "demo.txt", Content: []byte("demo"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	current, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	amount, term, purpose := int64(10000), 12, "WORKING_CAPITAL"
	if _, err := f.repo.Update(t.Context(), model.UpdateApplicationParams{
		OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: current.Version,
		Patch: model.ApplicationPatch{AmountKopecks: &amount, AmountCurrency: "RUB", TermMonths: &term, Purpose: &purpose},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := f.repo.Submit(t.Context(), model.SubmitApplicationParams{
		OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (f applicationFixture) event(t *testing.T, eventType string) *eventspb.Event {
	t.Helper()
	var payload []byte
	if err := f.db.QueryRow(t.Context(), "SELECT payload FROM outbox WHERE aggregate_id = $1 AND event_type = $2", f.draft.ID, eventType).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	event := new(eventspb.Event)
	if err := proto.Unmarshal(payload, event); err != nil {
		t.Fatal(err)
	}
	return event
}

func (f applicationFixture) project(t *testing.T, version int64, state *eventspb.ApplicationPublicStatePublished) *eventspb.Event {
	t.Helper()
	event := bankStateEvent(f.draft.ID, version, state)
	if applied, err := bankevents.NewProjection(f.db).Execute(t.Context(), event); err != nil || !applied {
		t.Fatalf("project bank event: applied=%v, err=%v", applied, err)
	}
	return event
}

func bankStateEvent(id uuid.UUID, version int64, state *eventspb.ApplicationPublicStatePublished) *eventspb.Event {
	return &eventspb.Event{
		EventId: uuid.NewString(), AggregateType: "APPLICATION", AggregateId: id.String(),
		AggregateVersion: version, CorrelationId: uuid.NewString(), OccurredAt: timestamppb.Now(),
		Payload: &eventspb.Event_ApplicationPublicStatePublished{ApplicationPublicStatePublished: state},
	}
}

func TestCreateReadAndSubmitIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	stored, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil || stored.Company.INN != "7707083893" || stored.Company.RegisteredAt.Format("2006-01-02") != "2020-01-02" {
		t.Fatalf("read created draft: company=%+v, err=%v", stored.Company, err)
	}
	if got := f.submit(t); got.Status != "SUBMITTED" {
		t.Fatalf("status = %s", got.Status)
	}
	submitted := f.event(t, "ApplicationSubmitted").GetApplicationSubmitted()
	if submitted.GetCompany().GetInn() != stored.Company.INN || submitted.GetForm().GetPurpose() != commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL {
		t.Fatalf("incorrect submitted snapshot: %v", submitted)
	}
	seen := make(map[commonpb.DocumentType]bool)
	for _, document := range submitted.GetDocuments() {
		seen[document.GetDocumentType()] = true
	}
	if len(seen) != 2 || !seen[commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT] || !seen[commonpb.DocumentType_DOCUMENT_TYPE_BUSINESS_PLAN] {
		t.Fatalf("document types = %v", seen)
	}
	var unlinked int
	if err := f.db.QueryRow(t.Context(), "SELECT count(*) FROM idempotency_keys WHERE application_id IS NULL").Scan(&unlinked); err != nil || unlinked != 0 {
		t.Fatalf("unlinked idempotency records = %d, err=%v", unlinked, err)
	}
}

func TestActionsHideMissingAndForeignApplicationsIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	hash := sha256.Sum256([]byte("action"))
	for _, target := range []struct {
		name string
		id   uuid.UUID
		org  uuid.UUID
	}{
		{"missing", uuid.New(), f.params.OrgID},
		{"foreign", f.draft.ID, uuid.New()},
	} {
		for name, action := range map[string]func(context.Context) error{
			"cancel": func(ctx context.Context) error {
				_, err := f.repo.CancelApplication(ctx, model.CancelApplicationParams{OrgID: target.org, UserID: f.params.UserID, ApplicationID: target.id, IdempotencyID: uuid.New(), RequestHash: hash[:]})
				return err
			},
			"submit": func(ctx context.Context) error {
				_, err := f.repo.Submit(ctx, model.SubmitApplicationParams{OrgID: target.org, UserID: f.params.UserID, ApplicationID: target.id, IdempotencyID: uuid.New()})
				return err
			},
			"document": func(ctx context.Context) error {
				_, err := f.repo.AddDocument(ctx, model.AddApplicationDocumentParams{OrgID: target.org, UserID: f.params.UserID, ApplicationID: target.id, IdempotencyKey: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "demo.txt"})
				return err
			},
			"info": func(ctx context.Context) error {
				_, err := f.repo.ProvideInfo(ctx, model.ProvideInfoParams{OrgID: target.org, UserID: f.params.UserID, ApplicationID: target.id, IdempotencyID: uuid.New(), RequestHash: hash[:]})
				return err
			},
			"accept": func(ctx context.Context) error {
				_, err := f.repo.AcceptOffer(ctx, model.AcceptOfferParams{OrgID: target.org, UserID: f.params.UserID, ApplicationID: target.id, IdempotencyID: uuid.New(), RequestHash: hash[:]})
				return err
			},
			"decline": func(ctx context.Context) error {
				_, err := f.repo.DeclineOffer(ctx, model.DeclineOfferParams{OrgID: target.org, UserID: f.params.UserID, ApplicationID: target.id, IdempotencyID: uuid.New(), RequestHash: hash[:]})
				return err
			},
			"sign": func(ctx context.Context) error {
				_, err := f.repo.SignContract(ctx, model.SignContractParams{OrgID: target.org, UserID: f.params.UserID, ApplicationID: target.id, IdempotencyID: uuid.New(), RequestHash: hash[:]})
				return err
			},
		} {
			t.Run(target.name+"/"+name, func(t *testing.T) {
				if err := action(t.Context()); !errors.Is(err, domain.ErrApplicationNotFound) {
					t.Fatalf("err = %v, want application not found", err)
				}
			})
		}
	}
	var count int
	if err := f.db.QueryRow(t.Context(), "SELECT count(*) FROM idempotency_keys").Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed actions left idempotency records: count=%d, err=%v", count, err)
	}
}

func TestInfoPendingAndReplayIntegration(t *testing.T) {
	f := newApplicationFixture(t)
	f.submit(t)
	requestID := uuid.New()
	f.project(t, 1, &eventspb.ApplicationPublicStatePublished{
		Status: eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO, StageLabel: "Нужны данные",
		ActiveInfoRequest: &commonpb.InfoRequest{RequestId: requestID.String(), Message: "Уточните сумму", CreatedAt: timestamppb.Now(), RequiredFields: []commonpb.InfoField{commonpb.InfoField_INFO_FIELD_APPLICATION_AMOUNT}},
	})
	amount := int64(20000)
	hash := sha256.Sum256([]byte("info"))
	params := model.ProvideInfoParams{
		OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
		RequestID: requestID, IdempotencyID: uuid.New(), RequestHash: hash[:], Fields: &model.ApplicationPatch{AmountKopecks: &amount},
	}
	first, err := f.repo.ProvideInfo(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.repo.ProvideInfo(t.Context(), params)
	if err != nil || replay != first {
		t.Fatalf("replay = %+v, err=%v", replay, err)
	}
	params.IdempotencyID = uuid.New()
	if _, err := f.repo.ProvideInfo(t.Context(), params); !errors.Is(err, domain.ErrApplicationInfoConflict) {
		t.Fatalf("second command: err=%v", err)
	}
	info := f.event(t, "InfoProvided").GetInfoProvided()
	if info.GetForm().GetAmount().GetAmountKopecks() != amount || info.GetForm().GetPurpose() != commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL || len(info.GetDocuments()) != 2 {
		t.Fatalf("info event = %v", info)
	}
	for _, document := range info.GetDocuments() {
		if document.GetDocumentType() == commonpb.DocumentType_DOCUMENT_TYPE_UNSPECIFIED {
			t.Fatal("info contains unspecified document type")
		}
	}
	var version, outboxCount, timelineCount int
	if err := f.db.QueryRow(t.Context(), `SELECT client_event_version,
		(SELECT count(*) FROM outbox),
		(SELECT count(*) FROM timeline_entries WHERE action = 'INFO_SENT')
		FROM applications WHERE id = $1`, f.draft.ID).Scan(&version, &outboxCount, &timelineCount); err != nil || version != 2 || outboxCount != 2 || timelineCount != 1 {
		t.Fatalf("version=%d, outbox=%d, info timeline=%d, err=%v", version, outboxCount, timelineCount, err)
	}
}

func TestRepeatedPublicSnapshotsIntegration(t *testing.T) {
	for _, kind := range []string{"offer", "info"} {
		t.Run(kind, func(t *testing.T) {
			f := newApplicationFixture(t)
			f.submit(t)
			state := &eventspb.ApplicationPublicStatePublished{StageLabel: "Обновление"}
			if kind == "offer" {
				state.Status = eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE
				state.ActiveOffer = &commonpb.Offer{
					OfferId: uuid.NewString(), Kind: commonpb.OfferKind_OFFER_KIND_OFFER,
					ProductId: f.params.ProductID.String(), ProductVersion: 1, ProductName: "Демо-продукт",
					Amount: &commonpb.Money{AmountKopecks: 10000, Currency: "RUB"}, TermMonths: 12,
					RateBps: 1900, ExpiresAt: timestamppb.New(time.Now().Add(72 * time.Hour)),
				}
			} else {
				state.Status = eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO
				state.ActiveInfoRequest = &commonpb.InfoRequest{
					RequestId: uuid.NewString(), Message: "Уточните сумму", CreatedAt: timestamppb.Now(),
					RequiredFields: []commonpb.InfoField{commonpb.InfoField_INFO_FIELD_APPLICATION_AMOUNT},
				}
			}
			f.project(t, 1, state)
			hash := sha256.Sum256([]byte("offer action"))
			if kind == "offer" {
				params := model.AcceptOfferParams{
					OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
					OfferID: uuid.MustParse(state.ActiveOffer.GetOfferId()), IdempotencyID: uuid.New(), RequestHash: hash[:],
				}
				first, err := f.repo.AcceptOffer(t.Context(), params)
				if err != nil {
					t.Fatal(err)
				}
				if replay, err := f.repo.AcceptOffer(t.Context(), params); err != nil || replay != first {
					t.Fatalf("accept replay = %+v, err=%v", replay, err)
				}
			}
			state.StageLabel = "Новое состояние"
			if kind == "offer" {
				state.Status = eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING
			}
			event := f.project(t, 2, state)
			if applied, err := bankevents.NewProjection(f.db).Execute(t.Context(), event); err != nil || applied {
				t.Fatalf("duplicate bank event: applied=%v, err=%v", applied, err)
			}
			got, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
			if err != nil || got.StageLabel != state.StageLabel {
				t.Fatalf("projected application = %+v, err=%v", got, err)
			}
			if kind == "offer" && (got.Offer == nil || got.Offer.ID.String() != state.ActiveOffer.GetOfferId()) {
				t.Fatal("repeated snapshot lost active offer")
			}
			if kind == "info" && (got.InfoRequest == nil || got.InfoRequest.ID.String() != state.ActiveInfoRequest.GetRequestId()) {
				t.Fatal("repeated snapshot lost active info request")
			}
			if kind == "offer" {
				params := model.SignContractParams{
					OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID,
					OfferID: got.Offer.ID, IdempotencyID: uuid.New(), RequestHash: hash[:],
				}
				first, err := f.repo.SignContract(t.Context(), params)
				if err != nil {
					t.Fatal(err)
				}
				if replay, err := f.repo.SignContract(t.Context(), params); err != nil || replay != first {
					t.Fatalf("sign replay = %+v, err=%v", replay, err)
				}
				params.RequestHash = make([]byte, 32)
				if _, err := f.repo.SignContract(t.Context(), params); !errors.Is(err, domain.ErrIdempotencyConflict) {
					t.Fatalf("sign with changed body: err=%v", err)
				}
				signed := f.event(t, "ContractSigned")
				if signed.GetAggregateVersion() != 3 || signed.GetContractSigned().GetOfferId() != got.Offer.ID.String() || signed.GetContractSigned().GetSignedAt() == nil {
					t.Fatalf("signed event = %v", signed)
				}
				var unlinked int
				if err := f.db.QueryRow(t.Context(), "SELECT count(*) FROM idempotency_keys WHERE application_id IS NULL").Scan(&unlinked); err != nil || unlinked != 0 {
					t.Fatalf("unlinked offer/sign idempotency records = %d, err=%v", unlinked, err)
				}
			}
			otherParams := f.params
			otherParams.IdempotencyID = uuid.New()
			other, err := f.repo.Create(t.Context(), otherParams)
			if err != nil {
				t.Fatal(err)
			}
			// Give the second application a valid submitted state, then ensure an
			// incoming snapshot cannot steal the first application's child IDs.
			if _, err := f.db.Exec(t.Context(), `UPDATE applications SET status = 'SUBMITTED', requested_amount_kopecks = 10000,
			    requested_term_months = 12, purpose_tag = 'WORKING_CAPITAL', submitted_at = now(), client_event_version = 1 WHERE id = $1`, other.ID); err != nil {
				t.Fatal(err)
			}
			conflict := bankStateEvent(other.ID, 1, state)
			if _, err := bankevents.NewProjection(f.db).Execute(t.Context(), conflict); err == nil {
				t.Fatal("reused child ID on another application was accepted")
			}
			var bankVersion, processed int
			if err := f.db.QueryRow(t.Context(), `SELECT bank_event_version,
			    (SELECT count(*) FROM processed_events WHERE event_id = $2) FROM applications WHERE id = $1`, other.ID, conflict.GetEventId()).Scan(&bankVersion, &processed); err != nil || bankVersion != 0 || processed != 0 {
				t.Fatalf("failed projection was not atomic: bank version=%d, processed=%d, err=%v", bankVersion, processed, err)
			}
		})
	}
}
