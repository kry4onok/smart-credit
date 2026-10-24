package decisionrepository

import (
	"context"
	"os"
	"testing"
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/decision"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

const testProductQuery = `INSERT INTO products (id,version,code,name,purpose_tags,required_document_types,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,indicative_rate_bps,is_active) VALUES ($1,1,$2,'Test loan',ARRAY['WORKING_CAPITAL'],ARRAY['BANK_STATEMENT'],100000000,5000000000,6,36,1600,true)`
const testRulesQuery = `INSERT INTO product_rules (product_id,product_version,min_company_age_months,max_amount_to_revenue_bps,max_overdue_days,rate_grade_a_bps,rate_grade_b_bps,offer_ttl_hours) VALUES ($1,1,12,5000,30,1500,1600,72)`
const testCaseQuery = `INSERT INTO cases (id,org_id,client_user_id,product_id,product_version,product_name,company_inn,company_name,company_snapshot,application_snapshot,documents_snapshot,requested_amount_kopecks,requested_term_months,purpose_tag,status,client_event_version) VALUES ($1,$2,$3,$4,1,'Test loan','7707083893','Test company',$5,'{}'::jsonb,$6,300000000,18,'WORKING_CAPITAL','DECISION',1)`
const testScoreQuery = `INSERT INTO score_results (case_id,score,grade,reasons,features,model_version,feature_schema_version) VALUES ($1,720,'B','[]'::jsonb,'{}'::jsonb,'test','test') RETURNING id`
const testRowsQuery = `SELECT c.status,c.version,d.outcome,d.rule_version,d.score_result_id,o.rate_bps,o.status,(SELECT count(*) FROM audit_log WHERE case_id=c.id),(SELECT count(*) FROM outbox WHERE aggregate_id=c.id::text) FROM cases c JOIN decisions d ON d.case_id=c.id JOIN offers o ON o.case_id=c.id WHERE c.id=$1`
const testPayloadQuery = `SELECT payload FROM outbox WHERE aggregate_id=$1`

func TestAutomaticDecisionPersistsOfferAuditAndEvent(t *testing.T) {
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
	company := []byte(`{"inn":"7707083893","name":"Test company","registered_at":"2019-04-15","revenue_12m":{"amount_kopecks":840000000,"currency":"RUB"},"profit_12m":{"amount_kopecks":96000000,"currency":"RUB"},"obligations":{"amount_kopecks":120000000,"currency":"RUB"}}`)
	documents := []byte(`[{"document_id":"40000000-0000-4000-8000-000000000001","document_type":"BANK_STATEMENT","file_name":"statement.pdf","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]`)
	if _, err = tx.Exec(ctx, testProductQuery, productID, "AUTO-TEST-"+productID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testRulesQuery, productID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, testCaseQuery, caseID, uuid.New(), uuid.New(), productID, company, documents); err != nil {
		t.Fatal(err)
	}
	var scoreID int64
	if err = tx.QueryRow(ctx, testScoreQuery, caseID).Scan(&scoreID); err != nil {
		t.Fatal(err)
	}
	in := decision.Input{
		Snapshot: validation.Snapshot{CompanyJSON: company, DocumentsJSON: documents, AmountKopecks: 300000000, TermMonths: 18, PurposeTag: "WORKING_CAPITAL"},
		Product:  decision.Product{Found: true, Name: "Test loan", Version: 1, Purposes: []string{"WORKING_CAPITAL"}, Documents: []string{"BANK_STATEMENT"}, MinAmount: 100000000, MaxAmount: 5000000000, MinTerm: 6, MaxTerm: 36, MinAgeMonths: 12, MaxAmountRevenueBP: 5000, MaxOverdueDays: 30, GradeARateBP: 1500, GradeBRateBP: 1600, OfferTTLHours: 72},
		Grade:    "B", ScoreFound: true, CompanyStatus: "ACTIVE", CompanyOK: true, CreditOK: true, Now: time.Now().UTC(),
	}
	result := decision.Evaluate(in)
	if err = complete(ctx, tx, caseID, productID, 1, &scoreID, in, result); err != nil {
		t.Fatal(err)
	}
	var status, outcome, offerStatus string
	var version, ruleVersion, storedScoreID int64
	var rate, audits, events int
	if err = tx.QueryRow(ctx, testRowsQuery, caseID).Scan(&status, &version, &outcome, &ruleVersion, &storedScoreID, &rate, &offerStatus, &audits, &events); err != nil {
		t.Fatal(err)
	}
	if status != "OFFER" || version != 2 || outcome != "APPROVE" || ruleVersion != 1 || storedScoreID != scoreID || rate != 1600 || offerStatus != "AVAILABLE" || audits != 1 || events != 1 {
		t.Fatalf("stored decision: status=%s version=%d outcome=%s rate=%d audits=%d events=%d", status, version, outcome, rate, audits, events)
	}
	var payload []byte
	if err = tx.QueryRow(ctx, testPayloadQuery, caseID.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var event eventspb.Event
	if err = proto.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.GetAggregateVersion() != 2 || event.GetApplicationPublicStatePublished().GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE || event.GetApplicationPublicStatePublished().GetActiveOffer().GetRateBps() != 1600 {
		t.Fatalf("public event = %v", &event)
	}
}
