package validationrepository

import (
	"context"
	"os"
	"testing"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	caserepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/case"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const insertValidationV2ProductQuery = `
INSERT INTO products (id,version,code,name,purpose_tags,min_amount_kopecks,max_amount_kopecks,
    min_term_months,max_term_months,indicative_rate_bps,requirements_policy_version)
VALUES ($1,1,$2,'v2 validation test',ARRAY['WORKING_CAPITAL'],10000000,500000000,6,36,1800,'sme-profile-v1')`
const insertValidationV2CaseQuery = `
INSERT INTO cases (id,org_id,client_user_id,product_id,product_version,product_name,
    company_inn,company_name,company_snapshot,application_snapshot,documents_snapshot,
    requested_amount_kopecks,requested_term_months,purpose_tag,status,next_run_at,
    client_event_version,requirements_reference_date,created_at)
VALUES ($1,$2,$3,$4,1,'v2 validation test','990000000115','ИП Демо',$5::jsonb,$6::jsonb,'[]'::jsonb,
    100000000,24,'WORKING_CAPITAL','VALIDATION',now(),1,'2026-10-26','1900-01-01'::timestamptz)`
const readValidationV2RequestQuery = `SELECT requirements,required_fields,required_document_types FROM info_requests WHERE case_id=$1 AND status='OPEN'`
const readValidationV2OutboxQuery = `SELECT payload FROM outbox WHERE aggregate_id=$1 AND aggregate_version=2`

func TestV2ValidationPersistsRequirementsAndPublicPlan(t *testing.T) {
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
	productID, caseID := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, insertValidationV2ProductQuery, productID, "V2-VALIDATION-"+productID.String()); err != nil {
		t.Fatal(err)
	}
	company := `{"inn":"990000000115","name":"ИП Демо","okved":"47.11","registered_at":"2020-01-01","profile_schema_version":2,"legal_form":"LEGAL_FORM_IP","tax_periods":[{"from":"2020-01-01","regimes":["TAX_REGIME_USN_INCOME"],"vat_status":"VAT_STATUS_UNKNOWN"}],"revenue_12m":{"amount_kopecks":"100000000","currency":"RUB"},"profit_12m":{"amount_kopecks":"0","currency":"RUB"},"obligations":{"amount_kopecks":"0","currency":"RUB"},"financial_period":{"from":"2025-10-01","to":"2026-09-30"},"financial_basis":"FINANCIAL_BASIS_MANAGEMENT","obligations_as_of":"2026-10-20"}`
	form := `{"amount":{"amount_kopecks":"100000000","currency":"RUB"},"term_months":24,"purpose":"PURPOSE_TAG_WORKING_CAPITAL"}`
	if _, err := tx.Exec(ctx, insertValidationV2CaseQuery, caseID, uuid.New(), uuid.New(), productID, company, form); err != nil {
		t.Fatal(err)
	}
	repository := New(tx)
	claim, found, err := repository.ClaimNext(ctx)
	if err != nil || !found || claim.CaseID != caseID {
		t.Fatalf("claim=%+v, found=%v, err=%v", claim, found, err)
	}
	product, err := repository.LoadProduct(ctx, claim)
	if err != nil || !product.Found || product.PolicyVersion != "sme-profile-v1" {
		t.Fatalf("product=%+v, err=%v", product, err)
	}
	outcome := validation.Evaluate(claim.Snapshot, product)
	if outcome.Status != model.CaseStatusNeedsInfo || len(outcome.RequestedRequirements) != 2 {
		t.Fatalf("outcome=%+v", outcome)
	}
	if err := repository.Complete(ctx, claim, outcome); err != nil {
		t.Fatal(err)
	}
	var requirementsJSON []byte
	var fields, types []string
	if err := tx.QueryRow(ctx, readValidationV2RequestQuery, caseID).Scan(&requirementsJSON, &fields, &types); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 0 || len(types) != 0 || len(requirementsJSON) <= 2 {
		t.Fatalf("request=%s, fields=%v, types=%v", requirementsJSON, fields, types)
	}
	var payload []byte
	if err := tx.QueryRow(ctx, readValidationV2OutboxQuery, caseID.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	event := new(eventspb.Event)
	if err := proto.Unmarshal(payload, event); err != nil {
		t.Fatal(err)
	}
	state := event.GetApplicationPublicStatePublished()
	if state.GetRequirementsPlan() == nil || state.GetRequirementsPlan().CanSubmit || len(state.GetActiveInfoRequest().GetRequirements()) != 2 {
		t.Fatalf("public state=%v", state)
	}
	item, found, err := caserepository.New(tx).GetCase(ctx, caseID)
	if err != nil || !found {
		t.Fatalf("GetCase: %v, found=%v", err, found)
	}
	if item.CompanyV2 == nil || item.CompanyV2.Profit_12M == nil || item.RequirementsPlan == nil || item.InfoRequest == nil || len(item.InfoRequest.Requirements) != 2 {
		t.Fatalf("Bank GET model=%+v", item)
	}
}
