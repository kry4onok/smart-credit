package receiverepository

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertV2TestProductQuery = `
INSERT INTO products (id,version,code,name,purpose_tags,min_amount_kopecks,max_amount_kopecks,
    min_term_months,max_term_months,indicative_rate_bps,requirements_policy_version)
VALUES ($1,1,$2,'v2 test',ARRAY['WORKING_CAPITAL'],10000000,500000000,6,36,1800,'sme-profile-v1')`
const setV2TestNeedsInfoQuery = `UPDATE cases SET status='NEEDS_INFO', next_run_at=NULL WHERE id=$1`
const readV2TestStateQuery = `SELECT status,client_event_version,requirements_reference_date,documents_snapshot FROM cases WHERE id=$1`
const readV2OpenRequestQuery = `SELECT id FROM info_requests WHERE case_id=$1 AND status='OPEN'`
const countV2OutboxQuery = `SELECT count(*) FROM outbox WHERE aggregate_id=$1`

func TestV2InfoProvidedReissuesIncompleteRequestAndPreservesDocuments(t *testing.T) {
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
	repository := New(tx)
	productID, caseID := uuid.New(), uuid.New()
	if _, err := tx.Exec(ctx, insertV2TestProductQuery, productID, "V2-EVENT-"+productID.String()); err != nil {
		t.Fatal(err)
	}
	referenceDate := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	company := []byte(`{"inn":"990000000115","name":"ИП Демо","okved":"47.11","registered_at":"2020-01-01","profile_schema_version":2,"legal_form":"LEGAL_FORM_IP","tax_periods":[{"from":"2020-01-01","regimes":["TAX_REGIME_USN_INCOME"],"vat_status":"VAT_STATUS_UNKNOWN"}],"revenue_12m":{"amount_kopecks":"100000000","currency":"RUB"},"profit_12m":{"amount_kopecks":"0","currency":"RUB"},"obligations":{"amount_kopecks":"0","currency":"RUB"},"financial_period":{"from":"2025-10-01","to":"2026-09-30"},"financial_basis":"FINANCIAL_BASIS_MANAGEMENT","obligations_as_of":"2026-10-20"}`)
	form := []byte(`{"amount":{"amount_kopecks":"100000000","currency":"RUB"},"term_months":24,"purpose":"PURPOSE_TAG_WORKING_CAPITAL"}`)
	submitted := receive.Input{EventID: uuid.New(), ApplicationID: caseID, AggregateVersion: 1, OrgID: uuid.New(), ClientUserID: uuid.New(), ProductID: productID, ProductVersion: 1, RequirementsReferenceDate: &referenceDate, CompanyINN: "990000000115", CompanyName: "ИП Демо", ProfileSchemaVersion: 2, CompanyJSON: company, FormJSON: form, DocumentsJSON: []byte(`[]`), AmountKopecks: 100000000, TermMonths: 24, PurposeTag: "WORKING_CAPITAL"}
	if applied, err := repository.Apply(ctx, submitted); err != nil || !applied {
		t.Fatalf("submit: %v %v", applied, err)
	}
	if _, err := tx.Exec(ctx, setV2TestNeedsInfoQuery, caseID); err != nil {
		t.Fatal(err)
	}
	requestID := uuid.New()
	requirement := &commonpb.Requirement{Id: "cash_flow:2025-10-01:2026-09-30:all", Code: "CASH_FLOW", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT, RequiredAt: commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT, Status: commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING, AcceptedDocumentTypes: []commonpb.DocumentType{commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT}, Period: &commonpb.DatePeriod{From: "2025-10-01", To: "2026-09-30"}}
	requestJSON, err := encodeRequirements([]*commonpb.Requirement{requirement})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, insertV2InfoRequestQuery, requestID, caseID, incompleteInfoMessage, string(requestJSON)); err != nil {
		t.Fatal(err)
	}
	provided := submitted
	provided.InfoProvided = true
	provided.EventID = uuid.New()
	provided.AggregateVersion = 2
	provided.RequestID = requestID
	provided.DocumentsJSON = []byte(`[]`)
	if applied, err := repository.Apply(ctx, provided); err != nil || !applied {
		t.Fatalf("incomplete info: %v %v", applied, err)
	}
	var status string
	var eventVersion int64
	var pinnedDate time.Time
	var documentsJSON []byte
	if err := tx.QueryRow(ctx, readV2TestStateQuery, caseID).Scan(&status, &eventVersion, &pinnedDate, &documentsJSON); err != nil {
		t.Fatal(err)
	}
	if status != "NEEDS_INFO" || eventVersion != 2 || !pinnedDate.Equal(referenceDate) {
		t.Fatalf("state: %s/%d/%s", status, eventVersion, pinnedDate)
	}
	var nextRequestID uuid.UUID
	if err := tx.QueryRow(ctx, readV2OpenRequestQuery, caseID).Scan(&nextRequestID); err != nil {
		t.Fatal(err)
	}
	if nextRequestID == requestID {
		t.Fatal("incomplete answer reused request ID")
	}
	var outboxCount int
	if err := tx.QueryRow(ctx, countV2OutboxQuery, caseID.String()).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox=%d", outboxCount)
	}
	document := receive.DocumentSnapshot{DocumentID: uuid.NewString(), DocumentType: "BANK_STATEMENT", UploadedAt: referenceDate, RequirementID: requirement.Id, CoveragePeriod: requirement.Period}
	provided.EventID = uuid.New()
	provided.AggregateVersion = 3
	provided.RequestID = nextRequestID
	provided.DocumentsJSON, err = json.Marshal([]receive.DocumentSnapshot{document})
	if err != nil {
		t.Fatal(err)
	}
	if applied, err := repository.Apply(ctx, provided); err != nil || !applied {
		t.Fatalf("complete info: %v %v", applied, err)
	}
	if applied, err := repository.Apply(ctx, provided); err != nil || applied {
		t.Fatalf("replay: %v %v", applied, err)
	}
	if err := tx.QueryRow(ctx, readV2TestStateQuery, caseID).Scan(&status, &eventVersion, &pinnedDate, &documentsJSON); err != nil {
		t.Fatal(err)
	}
	var saved []receive.DocumentSnapshot
	if err := json.Unmarshal(documentsJSON, &saved); err != nil {
		t.Fatal(err)
	}
	if status != "VALIDATION" || eventVersion != 3 || !pinnedDate.Equal(referenceDate) || len(saved) != 1 || saved[0].DocumentID != document.DocumentID {
		t.Fatalf("final state: %s/%d/%s/%v", status, eventVersion, pinnedDate, saved)
	}
}
