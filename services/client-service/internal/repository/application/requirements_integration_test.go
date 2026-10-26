package application

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	productrepo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/product"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	fixtures "github.com/alfabank-hackathon-2026/smart-credit/seed/profile-requirements"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/consumer/bankevents"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"
	companyrepo "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/repository/company"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/testutil"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func fixtureProfile(t *testing.T, file string) model.Company {
	t.Helper()
	raw, err := fixtures.Files.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var c common.CompanyProfile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return profile.FromHTTP(c)
}
func newV2Fixture(t *testing.T) applicationFixture {
	t.Helper()
	db := testutil.Postgres(t)
	params := model.CreateApplicationParams{OrgID: uuid.New(), UserID: uuid.New(), ProductID: uuid.New(), IdempotencyID: uuid.New()}
	company := fixtureProfile(t, "company-ip-psn.json")
	company.Revenue12mKopecks = nil
	company.Profit12mKopecks = nil
	company.ObligationsKopecks = nil
	company.FinancialPeriod = nil
	company.ObligationsAsOf = nil
	company.TaxPeriods = nil
	if _, err := companyrepo.New(db).Create(t.Context(), params.OrgID, company); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), `INSERT INTO products(id,version,code,name,purpose_tags,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,required_document_types,requirements_policy_version) VALUES($1,1,'V2','Профиль',ARRAY['WORKING_CAPITAL'],10000000,500000000,6,36,'{}','sme-profile-v1')`, params.ProductID); err != nil {
		t.Fatal(err)
	}
	repo := New(Adapt(db), t.TempDir())
	draft, err := repo.Create(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	if draft.RequirementsPlan == nil || draft.RequirementsPlan.CanSubmit || draft.Company.Profit12mKopecks != nil {
		t.Fatalf("incomplete draft = %+v", draft)
	}
	if draft.RequirementsReferenceDate == nil || draft.RequirementsReferenceDate.Format(time.DateOnly) != draft.CreatedAt.UTC().Format(time.DateOnly) {
		t.Fatal("draft reference date is not pinned")
	}
	if _, err := db.Exec(t.Context(), "UPDATE applications SET requirements_reference_date='2026-10-26' WHERE id=$1", draft.ID); err != nil {
		t.Fatal(err)
	}
	return applicationFixture{db: db, repo: repo, params: params, draft: draft}
}
func (f applicationFixture) completeV2(t *testing.T) model.Application {
	t.Helper()
	c := fixtureProfile(t, "company-ip-psn.json")
	amount := int64(100000000)
	term := 12
	purpose := "WORKING_CAPITAL"
	app, err := f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: f.draft.ID, ExpectedVersion: f.draft.Version, Patch: model.ApplicationPatch{Company: &c, AmountKopecks: &amount, AmountCurrency: "RUB", TermMonths: &term, Purpose: &purpose}})
	if err != nil {
		t.Fatal(err)
	}
	if !app.RequirementsPlan.GetCanSubmit() {
		t.Fatalf("completed draft cannot submit: %v", app.RequirementsPlan)
	}
	return app
}
func (f applicationFixture) submitV2(t *testing.T) model.Application {
	t.Helper()
	f.completeV2(t)
	params := model.SubmitApplicationParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyID: uuid.New()}
	app, err := f.repo.Submit(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.repo.Submit(t.Context(), params)
	if err != nil || replay.Version != app.Version {
		t.Fatalf("submit replay=%+v error=%v", replay, err)
	}
	if len(app.Documents) != 0 || app.Status != "SUBMITTED" {
		t.Fatal("submit must not require assessment files")
	}
	var raw []byte
	if err := f.db.QueryRow(t.Context(), "SELECT payload FROM outbox WHERE aggregate_id=$1", app.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	event := &eventspb.Event{}
	if err := proto.Unmarshal(raw, event); err != nil {
		t.Fatal(err)
	}
	v := event.GetApplicationSubmitted()
	if v.GetRequirementsReferenceDate() != "2026-10-26" || v.Company.GetLegalForm() != commonpb.LegalForm_LEGAL_FORM_IP || len(v.Company.TaxPeriods) == 0 {
		t.Fatalf("submitted payload lost profile: %v", v)
	}
	return app
}
func (f applicationFixture) requestV2(t *testing.T, version int64, plan *commonpb.RequirementsPlan, items []*commonpb.Requirement) (*commonpb.InfoRequest, *eventspb.Event) {
	t.Helper()
	request := &commonpb.InfoRequest{RequestId: uuid.NewString(), Message: "Уточните данные", CreatedAt: timestamppb.Now(), Requirements: items}
	event := &eventspb.Event{EventId: uuid.NewString(), AggregateType: "APPLICATION", AggregateId: f.draft.ID.String(), AggregateVersion: version, OccurredAt: timestamppb.Now(), CorrelationId: uuid.NewString(), Payload: &eventspb.Event_ApplicationPublicStatePublished{ApplicationPublicStatePublished: &eventspb.ApplicationPublicStatePublished{Status: eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO, StageLabel: "Нужны данные", NextAction: "Ответьте банку", ActiveInfoRequest: request, RequirementsPlan: plan}}}
	applied, err := bankevents.NewProjection(f.db).Execute(t.Context(), event)
	if err != nil || !applied {
		t.Fatalf("projection: %v applied=%v", err, applied)
	}
	return request, event
}
func (f applicationFixture) uploadV2(t *testing.T, r *commonpb.Requirement) model.ApplicationDocument {
	t.Helper()
	from, err := time.Parse(time.DateOnly, r.Period.From)
	if err != nil {
		t.Fatal(err)
	}
	to, err := time.Parse(time.DateOnly, r.Period.To)
	if err != nil {
		t.Fatal(err)
	}
	params := model.AddApplicationDocumentParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, IdempotencyKey: uuid.New(), DocumentType: strings.TrimPrefix(r.AcceptedDocumentTypes[0].String(), "DOCUMENT_TYPE_"), FileName: "demo.pdf", Content: []byte("test-content"), RequirementID: profile.Ptr(r.Id), CoverageFrom: &from, CoverageTo: &to}
	doc, err := f.repo.AddDocument(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.repo.AddDocument(t.Context(), params)
	if err != nil || replay.ID != doc.ID {
		t.Fatalf("document replay: %v", err)
	}
	wider := to.AddDate(0, 0, 1)
	params.CoverageTo = &wider
	if _, err := f.repo.AddDocument(t.Context(), params); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("coverage not hashed: %v", err)
	}
	return doc
}
func (f applicationFixture) infoParams(request *commonpb.InfoRequest) model.ProvideInfoParams {
	hash := sha256.Sum256([]byte(request.RequestId))
	return model.ProvideInfoParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: f.draft.ID, RequestID: uuid.MustParse(request.RequestId), IdempotencyID: uuid.New(), DocumentsSpecified: true, RequestHash: hash[:]}
}

func TestOrphanBankEventDoesNotBlockNextApplication(t *testing.T) {
	f := newV2Fixture(t)
	app := f.submitV2(t)
	projection := bankevents.NewProjection(f.db)
	publicState := &eventspb.ApplicationPublicStatePublished{
		Status:     eventspb.ClientStatus_CLIENT_STATUS_UNDER_REVIEW,
		StageLabel: "На рассмотрении", NextAction: "Ожидайте решения банка",
	}
	makeEvent := func(id uuid.UUID, version int64) *eventspb.Event {
		return &eventspb.Event{
			EventId: uuid.NewString(), AggregateType: "APPLICATION", AggregateId: id.String(),
			AggregateVersion: version, OccurredAt: timestamppb.Now(), CorrelationId: app.ID.String(),
			Payload: &eventspb.Event_ApplicationPublicStatePublished{ApplicationPublicStatePublished: publicState},
		}
	}
	orphan := makeEvent(uuid.New(), 2)
	if applied, err := projection.Execute(t.Context(), orphan); err != nil || applied {
		t.Fatalf("orphan applied=%v err=%v", applied, err)
	}
	var recorded int
	if err := f.db.QueryRow(t.Context(), "SELECT count(*) FROM processed_events WHERE event_id=$1", orphan.EventId).Scan(&recorded); err != nil || recorded != 1 {
		t.Fatalf("orphan recorded=%d err=%v", recorded, err)
	}
	if applied, err := projection.Execute(t.Context(), orphan); err != nil || applied {
		t.Fatalf("orphan replay applied=%v err=%v", applied, err)
	}
	valid := makeEvent(app.ID, 1)
	if applied, err := projection.Execute(t.Context(), valid); err != nil || !applied {
		t.Fatalf("next event applied=%v err=%v", applied, err)
	}
	got, err := f.repo.Get(t.Context(), f.params.OrgID, app.ID)
	if err != nil || got.Status != "UNDER_REVIEW" {
		t.Fatalf("next application status=%q err=%v", got.Status, err)
	}
}
func TestV2DraftSnapshotAndSubmitWithoutFiles(t *testing.T) {
	f := newV2Fixture(t)
	app := f.completeV2(t)
	global, err := companyrepo.New(f.db).Fetch(t.Context(), f.params.OrgID)
	if err != nil {
		t.Fatal(err)
	}
	if global.Profit12mKopecks != nil {
		t.Fatal("draft edit changed global company")
	}
	c := app.Company
	c.INN = "500100732259"
	_, err = f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: app.ID, ExpectedVersion: app.Version, Patch: model.ApplicationPatch{Company: &c}})
	if !errors.Is(err, domain.ErrProfileIdentityImmutable) {
		t.Fatalf("identity error=%v", err)
	}
	_, err = f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: app.ID, ExpectedVersion: 1, Patch: model.ApplicationPatch{Company: &app.Company}})
	if !errors.Is(err, domain.ErrApplicationVersionConflict) {
		t.Fatalf("stale version=%v", err)
	}
	params := model.SubmitApplicationParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: app.ID, IdempotencyID: uuid.New()}
	result, err := f.repo.Submit(t.Context(), params)
	if err != nil || result.Status != "SUBMITTED" {
		t.Fatalf("submit=%+v err=%v", result, err)
	}
}
func TestV2InfoAccumulationExplicitFieldsAndProjectionOrder(t *testing.T) {
	f := newV2Fixture(t)
	app := f.submitV2(t)
	company := app.Company
	ref := app.RequirementsReferenceDate
	var product productSnapshotJSON
	var raw []byte
	if err := f.db.QueryRow(t.Context(), "SELECT product_snapshot FROM applications WHERE id=$1", app.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &product); err != nil {
		t.Fatal(err)
	}
	plan, err := buildPlan(company, product, app.Form, nil, ref)
	if err != nil {
		t.Fatal(err)
	}
	var assessment []*commonpb.Requirement
	var extra *commonpb.Requirement
	for _, r := range plan.Items {
		if r.RequiredAt == commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT {
			assessment = append(assessment, r)
		}
		if r.RequiredAt == commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST && extra == nil {
			extra = r
		}
	}
	request, oldEvent := f.requestV2(t, 1, plan, assessment)
	for _, r := range assessment {
		f.uploadV2(t, r)
	}
	params := f.infoParams(request)
	if _, err := f.repo.ProvideInfo(t.Context(), params); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.ProvideInfo(t.Context(), params); err != nil {
		t.Fatalf("info replay %v", err)
	}
	if extra == nil {
		t.Fatal("fixture needs on-request requirement")
	}
	request, _ = f.requestV2(t, 2, plan, []*commonpb.Requirement{extra})
	last := f.uploadV2(t, extra)
	params = f.infoParams(request)
	params.DocumentIDs = []uuid.UUID{uuid.New()}
	if _, err := f.repo.ProvideInfo(t.Context(), params); !errors.Is(err, domain.ErrInvalidInfoResponse) {
		t.Fatalf("foreign document=%v", err)
	}
	params.DocumentIDs = []uuid.UUID{last.ID}
	if _, err := f.repo.ProvideInfo(t.Context(), params); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if err := f.db.QueryRow(t.Context(), "SELECT payload FROM outbox WHERE aggregate_id=$1 ORDER BY aggregate_version DESC LIMIT 1", app.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	event := &eventspb.Event{}
	if err := proto.Unmarshal(payload, event); err != nil {
		t.Fatal(err)
	}
	if len(event.GetInfoProvided().Documents) != len(assessment)+1 {
		t.Fatal("follow-up lost previous documents")
	}
	// Explicitly requested existing value must be sent again, not inferred from old data.
	var field *commonpb.Requirement
	for _, r := range plan.Items {
		if r.FieldPath == "company.profit_12m" {
			field = r
		}
	}
	request, _ = f.requestV2(t, 3, plan, []*commonpb.Requirement{field})
	params = f.infoParams(request)
	if _, err := f.repo.ProvideInfo(t.Context(), params); err == nil {
		t.Fatal("documents-only response satisfied FIELD")
	}
	params.Company = &company
	if _, err := f.repo.ProvideInfo(t.Context(), params); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(t.Context(), "SELECT payload FROM outbox WHERE aggregate_id=$1 ORDER BY aggregate_version DESC LIMIT 1", app.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if err := proto.Unmarshal(payload, event); err != nil {
		t.Fatal(err)
	}
	if ids := event.GetInfoProvided().AnsweredRequirementIds; len(ids) != 1 || ids[0] != field.Id {
		t.Fatalf("answered fields=%v", ids)
	}
	projection := bankevents.NewProjection(f.db)
	if applied, err := projection.Execute(t.Context(), oldEvent); err != nil || applied {
		t.Fatalf("duplicate=%v %v", applied, err)
	}
	stale := proto.Clone(oldEvent).(*eventspb.Event)
	stale.EventId = uuid.NewString()
	if _, err := projection.Execute(t.Context(), stale); err == nil {
		t.Fatal("conflicting version accepted")
	}
	// A newer full snapshot without a plan must clear the projection.
	clear := proto.Clone(oldEvent).(*eventspb.Event)
	clear.EventId = uuid.NewString()
	clear.AggregateVersion = 4
	clear.GetApplicationPublicStatePublished().RequirementsPlan = nil
	clear.GetApplicationPublicStatePublished().ActiveInfoRequest = nil
	clear.GetApplicationPublicStatePublished().Status = eventspb.ClientStatus_CLIENT_STATUS_UNDER_REVIEW
	if _, err := projection.Execute(t.Context(), clear); err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.Get(t.Context(), f.params.OrgID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RequirementsPlan != nil || got.InfoRequest != nil {
		t.Fatal("stale plan/request survived full state")
	}
}
func TestV2CorrectionRemovesPatentRequirement(t *testing.T) {
	f := newV2Fixture(t)
	app := f.submitV2(t)
	var product productSnapshotJSON
	var raw []byte
	if err := f.db.QueryRow(t.Context(), "SELECT product_snapshot FROM applications WHERE id=$1", app.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &product); err != nil {
		t.Fatal(err)
	}
	plan, err := buildPlan(app.Company, product, app.Form, nil, app.RequirementsReferenceDate)
	if err != nil {
		t.Fatal(err)
	}
	var patent *commonpb.Requirement
	for _, r := range plan.Items {
		if r.Code == "PATENT_RIGHT" {
			patent = r
		}
	}
	if patent == nil {
		t.Fatal("no patent")
	}
	request, _ := f.requestV2(t, 1, plan, []*commonpb.Requirement{patent})
	params := f.infoParams(request)
	corrected := fixtureProfile(t, "company-ip-usn.json")
	params.Company = &corrected
	if _, err := f.repo.ProvideInfo(t.Context(), params); err != nil {
		t.Fatalf("correction blocked by obsolete patent: %v", err)
	}
}

func TestV2DocumentCoverageVersionsAndOwnership(t *testing.T) {
	f := newV2Fixture(t)
	app := f.completeV2(t)
	var req *commonpb.Requirement
	for _, r := range app.RequirementsPlan.Items {
		if r.Kind == commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT {
			req = r
			break
		}
	}
	if req == nil {
		t.Fatal("no document requirement")
	}
	first := f.uploadV2(t, req)
	second := f.uploadV2(t, req)
	got, err := f.repo.Get(t.Context(), f.params.OrgID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || len(got.Documents) != 2 {
		t.Fatal("upload replaced previous version")
	}
	from, _ := time.Parse(time.DateOnly, req.Period.From)
	to, _ := time.Parse(time.DateOnly, req.Period.To)
	wrongFrom := from.AddDate(0, 0, 1)
	params := model.AddApplicationDocumentParams{OrgID: f.params.OrgID, UserID: f.params.UserID, ApplicationID: app.ID, IdempotencyKey: uuid.New(), DocumentType: strings.TrimPrefix(req.AcceptedDocumentTypes[0].String(), "DOCUMENT_TYPE_"), FileName: "demo.pdf", Content: []byte("content"), RequirementID: profile.Ptr(req.Id), CoverageFrom: &wrongFrom, CoverageTo: &to}
	if _, err := f.repo.AddDocument(t.Context(), params); err == nil {
		t.Fatal("incomplete coverage accepted")
	}
	params.CoverageFrom = &from
	params.RequirementID = profile.Ptr("other-application-requirement")
	if _, err := f.repo.AddDocument(t.Context(), params); err == nil {
		t.Fatal("unknown requirement accepted")
	}
	params.RequirementID = profile.Ptr(req.Id)
	params.OrgID = uuid.New()
	if _, err := f.repo.AddDocument(t.Context(), params); !errors.Is(err, domain.ErrApplicationNotFound) {
		t.Fatalf("foreign application=%v", err)
	}
}
func TestV2StaleRequestAndInvalidLoanCorrection(t *testing.T) {
	f := newV2Fixture(t)
	app := f.submitV2(t)
	var p productSnapshotJSON
	var raw []byte
	if err := f.db.QueryRow(t.Context(), "SELECT product_snapshot FROM applications WHERE id=$1", app.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	plan, err := buildPlan(app.Company, p, app.Form, nil, app.RequirementsReferenceDate)
	if err != nil {
		t.Fatal(err)
	}
	var field *commonpb.Requirement
	for _, r := range plan.Items {
		if r.FieldPath == "form.amount" {
			field = r
		}
	}
	request, _ := f.requestV2(t, 1, plan, []*commonpb.Requirement{field})
	params := f.infoParams(request)
	params.RequestID = uuid.New()
	if _, err := f.repo.ProvideInfo(t.Context(), params); !errors.Is(err, domain.ErrStaleInfoRequest) {
		t.Fatalf("stale request=%v", err)
	}
	params.RequestID = uuid.MustParse(request.RequestId)
	params.Fields = &model.ApplicationPatch{AmountKopecks: profile.Ptr(int64(1)), AmountCurrency: "RUB"}
	if _, err := f.repo.ProvideInfo(t.Context(), params); err == nil {
		t.Fatal("out-of-product loan accepted")
	}
}

func TestV2SameDocumentTypeAcrossTaxPeriods(t *testing.T) {
	f := newV2Fixture(t)
	app := f.completeV2(t)
	data, err := fixtures.Load()
	if err != nil {
		t.Fatal(err)
	}
	var company model.Company
	for _, s := range data.Cases {
		if s.ID == "regime_changed" {
			company = profile.FromHTTP(s.Profile)
		}
	}
	app, err = f.repo.Update(t.Context(), model.UpdateApplicationParams{OrgID: f.params.OrgID, ApplicationID: app.ID, ExpectedVersion: app.Version, Patch: model.ApplicationPatch{Company: &company}})
	if err != nil {
		t.Fatal(err)
	}
	var ledgers []*commonpb.Requirement
	for _, r := range app.RequirementsPlan.Items {
		if r.Code == "INCOME_LEDGER" {
			ledgers = append(ledgers, r)
		}
	}
	if len(ledgers) < 2 {
		t.Fatal("expected separate ledger requirements for tax periods")
	}
	for _, r := range ledgers {
		f.uploadV2(t, r)
	}
	got, err := f.repo.Get(t.Context(), f.params.OrgID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.RequirementsPlan.Items {
		if r.Code == "INCOME_LEDGER" && (r.Status != commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED || len(r.DocumentIds) != 1) {
			t.Fatalf("period-specific document missing: %v", r)
		}
	}
	if len(got.Documents) != len(ledgers) {
		t.Fatal("same-type documents collapsed")
	}
}

func TestV2CatalogProjectionAndUnknownPolicy(t *testing.T) {
	f := newV2Fixture(t)
	event := &eventspb.Event{EventId: uuid.NewString(), AggregateType: "CATALOG", AggregateId: "catalog", AggregateVersion: 1, OccurredAt: timestamppb.Now(), CorrelationId: uuid.NewString(), Payload: &eventspb.Event_ProductCatalogPublished{ProductCatalogPublished: &eventspb.ProductCatalogPublished{Products: []*commonpb.Product{{ProductId: f.params.ProductID.String(), Version: 2, Code: "V2", Name: "Профиль", PurposeTags: []commonpb.PurposeTag{commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL}, MinAmount: &commonpb.Money{AmountKopecks: 10000000, Currency: "RUB"}, MaxAmount: &commonpb.Money{AmountKopecks: 500000000, Currency: "RUB"}, MinTermMonths: 6, MaxTermMonths: 36, RequirementsPolicyVersion: "sme-profile-v1"}}}}}
	if _, err := bankevents.NewProjection(f.db).Execute(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	product, err := productrepo.New(f.db).Get(t.Context(), f.params.ProductID)
	if err != nil {
		t.Fatal(err)
	}
	if product.RequirementsPolicyVersion == nil || *product.RequirementsPolicyVersion != "sme-profile-v1" || product.Version != 2 {
		t.Fatal("catalog policy lost")
	}
	app, err := f.repo.Get(t.Context(), f.params.OrgID, f.draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if app.ProductVersion != 1 {
		t.Fatal("existing draft product changed")
	}
	if _, err := f.db.Exec(t.Context(), "UPDATE products SET requirements_policy_version='future-policy' WHERE id=$1", f.params.ProductID); err != nil {
		t.Fatal(err)
	}
	params := f.params
	params.IdempotencyID = uuid.New()
	if _, err := f.repo.Create(t.Context(), params); err == nil {
		t.Fatal("unknown policy entered legacy path")
	}
}
