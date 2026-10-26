package decisionrepository

import (
	"context"
	"os"
	"testing"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const testDecisionV2Product = `INSERT INTO products (id,version,code,name,purpose_tags,required_document_types,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,indicative_rate_bps,is_active,requirements_policy_version) VALUES ($1,1,$2,'V2 decision test',ARRAY['WORKING_CAPITAL'],ARRAY[]::text[],10000000,500000000,6,36,1800,true,'sme-profile-v1')`
const testDecisionV2Case = `INSERT INTO cases (id,org_id,client_user_id,product_id,product_version,product_name,company_inn,company_name,company_snapshot,application_snapshot,documents_snapshot,requested_amount_kopecks,requested_term_months,purpose_tag,status,client_event_version,requirements_reference_date,created_at) VALUES ($1,$2,$3,$4,1,'V2 decision test','990000000115','ИП Демо',$5::jsonb,'{}'::jsonb,'[]'::jsonb,100000000,24,'WORKING_CAPITAL','DECISION',1,'2026-10-26','1900-01-01'::timestamptz)`
const testDecisionV2Rows = `SELECT c.status,c.version,(SELECT count(*) FROM info_requests WHERE case_id=c.id),(SELECT count(*) FROM outbox WHERE aggregate_id=c.id::text) FROM cases c WHERE c.id=$1`
const testDecisionV2Request = `SELECT requirements FROM info_requests WHERE case_id=$1 AND status='OPEN'`

func TestV2WorkerPublishesRequirementsOnce(t *testing.T) {
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
	productID, caseID := uuid.New(), uuid.New()
	if _, err = tx.Exec(ctx, testDecisionV2Product, productID, "V2-WORKER-"+productID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testRulesQuery, productID); err != nil {
		t.Fatal(err)
	}
	company := `{"inn":"990000000115","name":"ИП Демо","okved":"47.11","registered_at":"2020-01-01","profile_schema_version":2,"legal_form":"LEGAL_FORM_IP","tax_periods":[{"from":"2020-01-01","regimes":["TAX_REGIME_USN_INCOME"],"vat_status":"VAT_STATUS_UNKNOWN"}],"revenue_12m":{"amount_kopecks":"100000000","currency":"RUB"},"profit_12m":{"amount_kopecks":"0","currency":"RUB"},"obligations":{"amount_kopecks":"0","currency":"RUB"},"financial_period":{"from":"2025-10-01","to":"2026-09-30"},"financial_basis":"FINANCIAL_BASIS_MANAGEMENT","obligations_as_of":"2026-10-20"}`
	if _, err = tx.Exec(ctx, testDecisionV2Case, caseID, uuid.New(), uuid.New(), productID, company); err != nil {
		t.Fatal(err)
	}
	repo := New(tx, transaction.New(tx))
	if found, err := repo.RunOnce(ctx); err != nil || !found {
		t.Fatalf("first run found=%v err=%v", found, err)
	}
	if found, err := repo.RunOnce(ctx); err != nil || found {
		t.Fatalf("repeat found=%v err=%v", found, err)
	}
	var status string
	var version int64
	var requests, events int
	if err = tx.QueryRow(ctx, testDecisionV2Rows, caseID).Scan(&status, &version, &requests, &events); err != nil {
		t.Fatal(err)
	}
	if status != "NEEDS_INFO" || version != 2 || requests != 1 || events != 1 {
		t.Fatalf("case=%s/%d requests=%d events=%d", status, version, requests, events)
	}
	var raw []byte
	if err = tx.QueryRow(ctx, testDecisionV2Request, caseID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) <= 2 {
		t.Fatalf("empty request=%s", raw)
	}
	var payload []byte
	if err = tx.QueryRow(ctx, testPayloadQuery, caseID.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	event := new(eventspb.Event)
	if err = proto.Unmarshal(payload, event); err != nil {
		t.Fatal(err)
	}
	state := event.GetApplicationPublicStatePublished()
	if state.GetRequirementsPlan() == nil || len(state.GetActiveInfoRequest().GetRequirements()) == 0 {
		t.Fatalf("public state=%v", state)
	}
}
