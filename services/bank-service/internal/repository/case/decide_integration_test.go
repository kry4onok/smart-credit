package caserepository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	assessmentrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/assessment"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/decide"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const testDecisionProduct = `INSERT INTO products (id,version,code,name,purpose_tags,required_document_types,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,indicative_rate_bps,is_active) VALUES ($1,1,$2,'Test loan',ARRAY['WORKING_CAPITAL'],ARRAY['BANK_STATEMENT'],100000000,5000000000,6,36,1600,true)`
const testDecisionRules = `INSERT INTO product_rules (product_id,product_version,min_company_age_months,max_amount_to_revenue_bps,max_overdue_days,rate_grade_a_bps,rate_grade_b_bps,offer_ttl_hours) VALUES ($1,1,12,5000,30,1500,1600,168)`
const testDecisionCase = `INSERT INTO cases (id,org_id,client_user_id,product_id,product_version,product_name,company_inn,company_name,company_snapshot,application_snapshot,documents_snapshot,requested_amount_kopecks,requested_term_months,purpose_tag,status,assignee_id,client_event_version) VALUES ($1,$2,$3,$4,1,'Test loan','7707083893','Test company',$5,'{}'::jsonb,$6,300000000,18,'WORKING_CAPITAL','MANUAL_REVIEW',$7,1)`
const testDecisionCompanyCheck = `INSERT INTO enrichment_results (case_id,source,outcome,result) VALUES ($1,'COMPANY','SUCCESS','{"status":"COMPANY_STATUS_ACTIVE"}'::jsonb)`
const testDecisionCreditCheck = `INSERT INTO enrichment_results (case_id,source,outcome,result) VALUES ($1,'CREDIT_HISTORY','SUCCESS','{"max_overdue_days":0}'::jsonb)`
const testDecisionCounts = `SELECT (SELECT count(*) FROM decisions WHERE case_id=$1),(SELECT count(*) FROM offers WHERE case_id=$1),(SELECT count(*) FROM audit_log WHERE case_id=$1),(SELECT count(*) FROM outbox WHERE aggregate_id=$2)`
const testDecisionPayload = `SELECT payload FROM outbox WHERE aggregate_id=$1 AND aggregate_version=2`

func TestDecidePersistsOfferAuditOutboxAndReplay(t *testing.T) {
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
	if _, err = tx.Exec(ctx, testDecisionProduct, productID, "DECISION-TEST-"+productID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testDecisionRules, productID); err != nil {
		t.Fatal(err)
	}
	company := `{"inn":"7707083893","name":"Test company","okved":"10.71","registered_at":"2019-04-15","revenue_12m":{"amount_kopecks":840000000,"currency":"RUB"},"profit_12m":{"amount_kopecks":96000000,"currency":"RUB"},"obligations":{"amount_kopecks":120000000,"currency":"RUB"}}`
	docs := `[{"document_id":"` + uuid.NewString() + `","document_type":"BANK_STATEMENT","file_name":"statement.pdf","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]`
	if _, err = tx.Exec(ctx, testDecisionCase, caseID, uuid.New(), uuid.New(), productID, company, docs, analystID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testDecisionCompanyCheck, caseID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testDecisionCreditCheck, caseID); err != nil {
		t.Fatal(err)
	}
	var asOf time.Time
	if err = tx.QueryRow(ctx, `SELECT assessment_as_of FROM cases WHERE id=$1`, caseID).Scan(&asOf); err != nil {
		t.Fatal(err)
	}
	zero := 0
	debt := int64(0)
	start := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(-1, 0, 0)
	sources := assessment.DemoSources("7707083893", "2019-04-15", "ACTIVE", assessment.CreditHistory{HistoricalMaxDPD: &zero, CurrentDPD: &zero, OverdueEpisodes: &zero, TotalDebtKopecks: &debt, PeriodFrom: start.Format(time.DateOnly), PeriodTo: start.AddDate(1, 0, -1).Format(time.DateOnly)}, asOf)
	for i := range sources.Financials.Months {
		receipts := int64(200000000)
		sources.Financials.Months[i].Receipts = &receipts
	}
	sourceJSON, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE cases SET assessment_sources=$2 WHERE id=$1`, caseID, sourceJSON); err != nil {
		t.Fatal(err)
	}
	input, err := assessmentrepository.LoadInput(ctx, tx, caseID)
	if err != nil {
		t.Fatal(err)
	}
	assessed := assessment.Evaluate(input)
	if assessed.Status != "READY_FOR_CONFIRMATION" {
		t.Fatalf("assessment fixture: %+v", assessed)
	}
	inputJSON, _ := json.Marshal(input)
	resultJSON, _ := json.Marshal(assessed)
	publicJSON, _ := json.Marshal(assessment.Public(assessed))
	if _, err = tx.Exec(ctx, `INSERT INTO assessments(id,case_id,input_revision,assessment_generation,input,result,public_result,valid_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, assessed.ID, caseID, assessed.InputRevision, assessed.Generation, inputJSON, resultJSON, publicJSON, assessed.ValidUntil); err != nil {
		t.Fatal(err)
	}
	repo := New(tx)
	in := decide.Input{CaseID: caseID, AnalystID: analystID, IdempotencyKey: uuid.New(), ExpectedVersion: 1, Outcome: "APPROVE", Comment: "Internal review only", ClientText: "Здравствуйте. Одобрено предложение.", Offer: &decide.Offer{ProductID: productID, AmountKopecks: 300000000, TermMonths: 18, RateBPS: 1500}}
	if preview, ready := repo.PrepareDecision(ctx, in); !ready || preview.RequestedProductName != "Test loan" || preview.OfferedProductName != "Test loan" {
		t.Fatalf("prepare decision: preview=%+v ready=%t", preview, ready)
	}
	result, err := repo.Decide(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "OFFER" || result.Version != 2 || result.Offer == nil || result.Offer.ProductID != productID ||
		result.Offer.Explanation != in.ClientText || result.PublicReason == nil || *result.PublicReason != in.ClientText {
		t.Fatalf("case = %+v", result)
	}
	replayInput := in
	replayInput.ClientText = "Другой ответ модели"
	replay, err := repo.Decide(ctx, replayInput)
	if err != nil || replay.Version != result.Version {
		t.Fatalf("replay = %+v, %v", replay, err)
	}
	changed := in
	changed.Comment = "Changed request"
	if _, err = repo.Decide(ctx, changed); !errors.Is(err, decide.ErrIdempotencyConflict) {
		t.Fatalf("changed replay = %v", err)
	}
	var decisions, offers, audits, events int
	if err = tx.QueryRow(ctx, testDecisionCounts, caseID, caseID.String()).Scan(&decisions, &offers, &audits, &events); err != nil {
		t.Fatal(err)
	}
	if decisions != 1 || offers != 1 || audits != 1 || events != 1 {
		t.Fatalf("rows = %d %d %d %d", decisions, offers, audits, events)
	}
	var payload []byte
	if err = tx.QueryRow(ctx, testDecisionPayload, caseID.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var event eventspb.Event
	if err = proto.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	state := event.GetApplicationPublicStatePublished()
	if state.GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE || state.GetActiveOffer().GetProductId() != productID.String() || state.GetPublicReason() != in.ClientText ||
		state.GetActiveOffer().GetExplanation() != in.ClientText || bytes.Contains(payload, []byte(in.Comment)) {
		t.Fatalf("public event = %v", &event)
	}
}
