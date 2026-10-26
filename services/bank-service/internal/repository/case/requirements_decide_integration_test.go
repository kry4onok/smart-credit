package caserepository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/decide"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const testV2DecisionProduct = `INSERT INTO products (id,version,code,name,purpose_tags,required_document_types,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,indicative_rate_bps,is_active,requirements_policy_version) VALUES ($1,1,$2,'V2 test',ARRAY['WORKING_CAPITAL'],ARRAY[]::text[],10000000,500000000,6,36,1800,true,'sme-profile-v1')`
const testV2DecisionCase = `INSERT INTO cases (id,org_id,client_user_id,product_id,product_version,product_name,company_inn,company_name,company_snapshot,application_snapshot,documents_snapshot,requested_amount_kopecks,requested_term_months,purpose_tag,status,assignee_id,client_event_version,requirements_reference_date) VALUES ($1,$2,$3,$4,1,'V2 test','990000000115','ИП Демо',$5::jsonb,'{}'::jsonb,'[]'::jsonb,100000000,24,'WORKING_CAPITAL','MANUAL_REVIEW',$6,1,'2026-10-26')`
const testV2DecisionRequest = `SELECT requirements,required_fields,required_document_types FROM info_requests WHERE case_id=$1 AND status='OPEN'`
const testV2DecisionRows = `SELECT (SELECT count(*) FROM decisions WHERE case_id=$1),(SELECT count(*) FROM info_requests WHERE case_id=$1),(SELECT count(*) FROM audit_log WHERE case_id=$1),(SELECT count(*) FROM outbox WHERE aggregate_id=$2)`

func TestV2ManualRequestSelectsFreshRequirementsAndReplays(t *testing.T) {
	url := os.Getenv("BANK_DATABASE_URL")
	if url == "" {
		t.Skip("BANK_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	productID, caseID, analystID := uuid.New(), uuid.New(), uuid.New()
	if _, err = tx.Exec(ctx, testV2DecisionProduct, productID, "V2-DECIDE-"+productID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testDecisionRules, productID); err != nil {
		t.Fatal(err)
	}
	company := []byte(`{"inn":"990000000115","name":"ИП Демо","okved":"47.11","registered_at":"2020-01-01","profile_schema_version":2,"legal_form":"LEGAL_FORM_IP","tax_periods":[{"from":"2020-01-01","regimes":["TAX_REGIME_USN_INCOME"],"vat_status":"VAT_STATUS_UNKNOWN"}],"revenue_12m":{"amount_kopecks":"100000000","currency":"RUB"},"profit_12m":{"amount_kopecks":"0","currency":"RUB"},"obligations":{"amount_kopecks":"0","currency":"RUB"},"financial_period":{"from":"2025-10-01","to":"2026-09-30"},"financial_basis":"FINANCIAL_BASIS_MANAGEMENT","obligations_as_of":"2026-10-20"}`)
	if _, err = tx.Exec(ctx, testV2DecisionCase, caseID, uuid.New(), uuid.New(), productID, company, analystID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testDecisionCompanyCheck, caseID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testDecisionCreditCheck, caseID); err != nil {
		t.Fatal(err)
	}
	ref := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	outcome := validation.Evaluate(validation.Snapshot{CompanyJSON: company, DocumentsJSON: []byte(`[]`), ReferenceDate: &ref, AmountKopecks: 100000000, TermMonths: 24, PurposeTag: "WORKING_CAPITAL"}, validation.Product{Found: true, PolicyVersion: "sme-profile-v1", PurposeTags: []string{"WORKING_CAPITAL"}, MinAmountKopecks: 10000000, MaxAmountKopecks: 500000000, MinTermMonths: 6, MaxTermMonths: 36})
	if outcome.Plan == nil || len(outcome.RequestedRequirements) == 0 {
		t.Fatalf("validation outcome=%+v", outcome)
	}
	id := outcome.RequestedRequirements[0].GetId()
	repo := New(tx)
	offerInput := decide.Input{CaseID: caseID, AnalystID: analystID, IdempotencyKey: uuid.New(), ExpectedVersion: 1,
		Outcome: "APPROVE", Comment: "Проверка комплекта", Offer: &decide.Offer{ProductID: productID, AmountKopecks: 10000000, TermMonths: 24, RateBPS: 1800}}
	if _, err = repo.Decide(ctx, offerInput); !errors.Is(err, decide.ErrValidation) {
		t.Fatalf("incomplete v2 offer error=%v", err)
	}
	legacyProductID := uuid.New()
	if _, err = tx.Exec(ctx, testDecisionProduct, legacyProductID, "V2-COUNTER-"+legacyProductID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testDecisionRules, legacyProductID); err != nil {
		t.Fatal(err)
	}
	counter := offerInput
	counter.IdempotencyKey = uuid.New()
	counter.Outcome = "COUNTER"
	counter.PublicReason = "Другие условия"
	counter.Offer = &decide.Offer{ProductID: legacyProductID, AmountKopecks: 100000000, TermMonths: 24, RateBPS: 1800}
	if _, err = repo.Decide(ctx, counter); !errors.Is(err, decide.ErrValidation) || !strings.Contains(err.Error(), "ASSESSMENT_REQUIRED") {
		t.Fatalf("incompatible counter policy error=%v", err)
	}
	bad := decide.Input{CaseID: caseID, AnalystID: analystID, IdempotencyKey: uuid.New(), ExpectedVersion: 1, Outcome: "REQUEST_INFO", Comment: "Нужны данные", InfoRequest: &decide.InfoRequest{Message: "Добавьте документ", V2: true, RequirementIDs: []string{"unknown"}}}
	if _, err = repo.Decide(ctx, bad); err == nil {
		t.Fatal("unknown requirement accepted")
	} else {
		var rule *requirements.RuleError
		if !errors.As(err, &rule) || rule.Code != requirements.UnknownRequirement {
			t.Fatalf("unknown ID error=%v", err)
		}
	}
	// The rejected attempt reserved a key in this test transaction; production rolls it back.
	in := bad
	in.IdempotencyKey = uuid.New()
	in.InfoRequest = &decide.InfoRequest{Message: "Добавьте документ", V2: true, RequirementIDs: []string{id}}
	result, err := repo.Decide(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "NEEDS_INFO" || result.InfoRequest == nil || len(result.InfoRequest.Requirements) != 1 {
		t.Fatalf("result=%+v", result)
	}
	// When the client answers, the case returns to the queue for any analyst.
	var assignee *uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT assignee_id FROM cases WHERE id=$1`, caseID).Scan(&assignee); err != nil || assignee != nil {
		t.Fatalf("assignee after REQUEST_INFO=%v err=%v", assignee, err)
	}
	replay, err := repo.Decide(ctx, in)
	if err != nil || replay.Version != result.Version {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	var raw []byte
	var fields, docs []string
	if err = tx.QueryRow(ctx, testV2DecisionRequest, caseID).Scan(&raw, &fields, &docs); err != nil {
		t.Fatal(err)
	}
	if len(raw) <= 2 || len(fields) != 0 || len(docs) != 0 {
		t.Fatalf("stored request=%s fields=%v docs=%v", raw, fields, docs)
	}
	var decisions, requests, audits, outbox int
	if err = tx.QueryRow(ctx, testV2DecisionRows, caseID, caseID.String()).Scan(&decisions, &requests, &audits, &outbox); err != nil {
		t.Fatal(err)
	}
	if decisions != 1 || requests != 1 || audits != 1 || outbox != 1 {
		t.Fatalf("rows=%d/%d/%d/%d", decisions, requests, audits, outbox)
	}
	var payload []byte
	if err = tx.QueryRow(ctx, testDecisionPayload, caseID.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	event := new(eventspb.Event)
	if err = proto.Unmarshal(payload, event); err != nil {
		t.Fatal(err)
	}
	state := event.GetApplicationPublicStatePublished()
	if state.GetRequirementsPlan() == nil || len(state.GetActiveInfoRequest().GetRequirements()) != 1 || state.GetActiveInfoRequest().GetRequirements()[0].GetId() != id {
		t.Fatalf("event=%v", state)
	}
}

func TestRejectCaseWithUnknownProduct(t *testing.T) {
	url := os.Getenv("BANK_DATABASE_URL")
	if url == "" {
		t.Skip("BANK_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	caseID, analystID := uuid.New(), uuid.New()
	// The receive consumer keeps an application for an unknown product as a manual case.
	company := []byte(`{"inn":"990000000115","name":"ИП Демо","okved":"47.11","registered_at":"2020-01-01"}`)
	if _, err = tx.Exec(ctx, testV2DecisionCase, caseID, uuid.New(), uuid.New(), uuid.New(), company, analystID); err != nil {
		t.Fatal(err)
	}
	repo := New(tx)
	in := decide.Input{CaseID: caseID, AnalystID: analystID, IdempotencyKey: uuid.New(), ExpectedVersion: 1,
		Outcome: "REJECT", Comment: "Продукт снят с продажи", PublicReason: "Выбранный продукт больше не доступен."}
	if _, ok := repo.PrepareDecision(ctx, in); !ok {
		t.Fatal("preview rejected a case with an unknown product")
	}
	result, err := repo.Decide(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "REJECTED" {
		t.Fatalf("status=%s", result.Status)
	}
}
