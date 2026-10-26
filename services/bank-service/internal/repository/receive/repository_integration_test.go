package receiverepository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	caserepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/case"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertNeedsInfoCaseQuery = `
INSERT INTO cases (id, org_id, client_user_id, product_id, product_version, product_name,
    company_inn, company_name, company_snapshot, application_snapshot, documents_snapshot,
    requested_amount_kopecks, requested_term_months, purpose_tag, status, recommendation,
    public_reason, triggered_rules, client_event_version, version, audit_version,
    scoring_input_revision)
VALUES ($1, $2, $3, $4, 1, 'Test', '7707083893', 'Original',
    '{"inn":"7707083893","name":"Original"}'::jsonb, '{}'::jsonb, '[]'::jsonb,
    1000, 12, 'WORKING_CAPITAL', 'NEEDS_INFO', 'REQUEST_INFO',
    'Missing data', ARRAY['MISSING_REQUIRED_DATA'], 1, 2, 1, 3)`

const insertOpenInfoRequestQuery = `
INSERT INTO info_requests (id, case_id, message, required_fields)
VALUES ($1, $2, 'Missing data', ARRAY['COMPANY_REVENUE_12M'])`

const selectInfoProvidedStateQuery = `
SELECT status, recommendation, public_reason, next_run_at IS NOT NULL,
    company_inn, company_name, company_snapshot, application_snapshot, documents_snapshot,
    requested_amount_kopecks, requested_term_months, purpose_tag,
    client_event_version, version, audit_version, scoring_input_revision,
    current_scoring_attempt_id
FROM cases WHERE id = $1`

const selectInfoRequestStateQuery = `
SELECT status, responded_at IS NOT NULL FROM info_requests WHERE id = $1`

const countInfoProvidedAuditQuery = `
SELECT count(*) FROM audit_log WHERE case_id = $1 AND action = 'INFO_PROVIDED'`

const countProcessedEventQuery = `
SELECT count(*) FROM processed_events WHERE event_id = $1`

const setValidationStatusQuery = `UPDATE cases SET status = 'VALIDATION' WHERE id = $1`

func TestApplyCommitsOneCaseAndAuditPerEvent(t *testing.T) {
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
	productID := uuid.New()
	_, err = tx.Exec(ctx, `
INSERT INTO products (
    id, version, code, name, purpose_tags, min_amount_kopecks,
    max_amount_kopecks, min_term_months, max_term_months, indicative_rate_bps
) VALUES ($1, 1, $2, 'Test product', ARRAY['WORKING_CAPITAL'], 1,
          1000000, 1, 36, 1000)`, productID, "RECEIVE-TEST-"+productID.String())
	if err != nil {
		t.Fatal(err)
	}
	input := receive.Input{
		EventID: uuid.New(), ApplicationID: uuid.New(), AggregateVersion: 1,
		OrgID: uuid.New(), ClientUserID: uuid.New(),
		ProductID: productID, ProductVersion: 1,
		CompanyINN: "7707083893", CompanyName: "Test",
		CompanyJSON:   []byte(`{"inn":"7707083893","name":"Test","registered_at":"2020-01-01","revenue_12m":{"amount_kopecks":"100","currency":"RUB"},"profit_12m":{"amount_kopecks":"10","currency":"RUB"},"obligations":{"amount_kopecks":"5","currency":"RUB"}}`),
		FormJSON:      []byte(`{"amount":{"amount_kopecks":"1000","currency":"RUB"},"term_months":12,"purpose":"PURPOSE_TAG_WORKING_CAPITAL"}`),
		DocumentsJSON: []byte(`[]`), AmountKopecks: 1000, TermMonths: 12, PurposeTag: "WORKING_CAPITAL",
	}
	applied, err := repository.Apply(ctx, input)
	if err != nil || !applied {
		t.Fatalf("first Apply = %v, %v", applied, err)
	}
	applied, err = repository.Apply(ctx, input)
	if err != nil || applied {
		t.Fatalf("duplicate Apply = %v, %v", applied, err)
	}
	var status string
	var scheduled bool
	var version int64
	if err := tx.QueryRow(ctx, `SELECT status, next_run_at IS NOT NULL, client_event_version FROM cases WHERE id=$1`,
		input.ApplicationID).Scan(&status, &scheduled, &version); err != nil {
		t.Fatal(err)
	}
	if status != "VALIDATION" || !scheduled || version != 1 {
		t.Fatalf("case state = %s, scheduled=%v, event version=%d", status, scheduled, version)
	}
	var caseCount, auditCount, eventCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM cases WHERE id=$1`, input.ApplicationID).Scan(&caseCount); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE case_id=$1`, input.ApplicationID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM processed_events WHERE event_id=$1`, input.EventID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if caseCount != 1 || auditCount != 1 || eventCount != 1 {
		t.Fatalf("rows after replay: cases=%d audit=%d events=%d", caseCount, auditCount, eventCount)
	}

	input.EventID = uuid.New()
	input.ApplicationID = uuid.New()
	input.ProductID = uuid.New()
	applied, err = repository.Apply(ctx, input)
	if err != nil || !applied {
		t.Fatalf("unknown product Apply = %v, %v", applied, err)
	}
	if err := tx.QueryRow(ctx, `SELECT status, next_run_at IS NOT NULL FROM cases WHERE id=$1`,
		input.ApplicationID).Scan(&status, &scheduled); err != nil {
		t.Fatal(err)
	}
	if status != "MANUAL_REVIEW" || scheduled {
		t.Fatalf("unknown product state = %s, scheduled=%v", status, scheduled)
	}
}

func TestApplyInfoProvidedUpdatesCaseAndDeduplicates(t *testing.T) {
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
	caseID, requestID, eventID := uuid.New(), uuid.New(), uuid.New()
	_, err = tx.Exec(ctx, insertNeedsInfoCaseQuery, caseID, uuid.New(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, insertOpenInfoRequestQuery, requestID, caseID)
	if err != nil {
		t.Fatal(err)
	}
	input := receive.Input{
		InfoProvided: true, RequestID: requestID, EventID: eventID,
		ApplicationID: caseID, AggregateVersion: 3,
		CompanyINN: "1234567890", CompanyName: "Updated",
		CompanyJSON:   []byte(`{"inn":"1234567890","name":"Updated","revenue_12m":{"amount_kopecks":"500","currency":"RUB"}}`),
		FormJSON:      []byte(`{"amount":{"amount_kopecks":"2000","currency":"RUB"},"term_months":24,"purpose":"PURPOSE_TAG_EQUIPMENT"}`),
		DocumentsJSON: []byte(`[{"document_id":"11111111-1111-4111-8111-111111111111","document_type":"BANK_STATEMENT"}]`),
		AmountKopecks: 2000, TermMonths: 24, PurposeTag: "EQUIPMENT",
	}

	// A failed event reservation is rolled back with its surrounding transaction.
	expectConflict := func(name string, invalid receive.Input) {
		t.Helper()
		if _, err := tx.Exec(ctx, "SAVEPOINT invalid_info"); err != nil {
			t.Fatal(err)
		}
		invalid.EventID = uuid.New()
		if applied, err := repository.Apply(ctx, invalid); applied || !errors.Is(err, ErrInfoProvidedConflict) {
			t.Fatalf("%s = %v, %v", name, applied, err)
		}
		if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT invalid_info"); err != nil {
			t.Fatal(err)
		}
	}
	invalid := input
	invalid.RequestID = uuid.New()
	expectConflict("wrong request", invalid)
	invalid = input
	invalid.AggregateVersion = 1
	expectConflict("stale version", invalid)
	if _, err := tx.Exec(ctx, "SAVEPOINT wrong_status"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, setValidationStatusQuery, caseID); err != nil {
		t.Fatal(err)
	}
	expectConflict("wrong status", input)
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT wrong_status"); err != nil {
		t.Fatal(err)
	}

	applied, err := repository.Apply(ctx, input)
	if err != nil || !applied {
		t.Fatalf("InfoProvided Apply = %v, %v", applied, err)
	}
	applied, err = repository.Apply(ctx, input)
	if err != nil || applied {
		t.Fatalf("replayed Apply = %v, %v", applied, err)
	}
	var status, inn, name, purpose string
	var recommendation, reason *string
	var companyJSON, formJSON, documentsJSON []byte
	var scheduled bool
	var amount, eventVersion, version, auditVersion, scoringRevision int64
	var term int32
	var attemptID *uuid.UUID
	err = tx.QueryRow(ctx, selectInfoProvidedStateQuery, caseID).Scan(
		&status, &recommendation, &reason, &scheduled,
		&inn, &name, &companyJSON, &formJSON, &documentsJSON,
		&amount, &term, &purpose, &eventVersion, &version, &auditVersion,
		&scoringRevision, &attemptID)
	if err != nil {
		t.Fatal(err)
	}
	if status != "VALIDATION" || recommendation != nil || reason != nil || !scheduled ||
		inn != input.CompanyINN || name != input.CompanyName || amount != input.AmountKopecks ||
		term != input.TermMonths || purpose != input.PurposeTag || eventVersion != 3 ||
		version != 3 || auditVersion != 2 || scoringRevision != 4 || attemptID != nil {
		t.Fatalf("case state = %s, %v, %v, scheduled=%v, %s, %s, %d, %d, %s, versions=%d/%d/%d/%d, attempt=%v",
			status, recommendation, reason, scheduled, inn, name, amount, term, purpose,
			eventVersion, version, auditVersion, scoringRevision, attemptID)
	}
	var company, form map[string]any
	var documents []map[string]any
	if err := json.Unmarshal(companyJSON, &company); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(formJSON, &form); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(documentsJSON, &documents); err != nil {
		t.Fatal(err)
	}
	if company["name"] != "Updated" || form["purpose"] != "PURPOSE_TAG_EQUIPMENT" ||
		len(documents) != 1 || documents[0]["document_type"] != "BANK_STATEMENT" {
		t.Fatalf("snapshots = %v, %v, %v", company, form, documents)
	}
	var requestStatus string
	var responded bool
	if err := tx.QueryRow(ctx, selectInfoRequestStateQuery, requestID).Scan(&requestStatus, &responded); err != nil {
		t.Fatal(err)
	}
	if requestStatus != "FULFILLED" || !responded {
		t.Fatalf("request = %s, responded=%v", requestStatus, responded)
	}
	var auditCount, eventCount int
	if err := tx.QueryRow(ctx, countInfoProvidedAuditQuery, caseID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, countProcessedEventQuery, eventID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 || eventCount != 1 {
		t.Fatalf("audit=%d, processed=%d", auditCount, eventCount)
	}
}

func TestOtherPurposeReachesBankCaseIntegration(t *testing.T) {
	databaseURL := os.Getenv("BANK_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("BANK_DATABASE_URL is not set")
	}
	ctx := t.Context()
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
	id, productID := uuid.New(), uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO products(id,version,code,name,purpose_tags,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,indicative_rate_bps) VALUES($1,1,$2,'Mixed purpose',ARRAY['OTHER'],1,1000000,1,36,1000)`, productID, productID.String()); err != nil {
		t.Fatal(err)
	}
	description := "Ремонт, оборудование и сырьё"
	formJSON, err := json.Marshal(map[string]any{"amount": map[string]any{"amount_kopecks": 1000, "currency": "RUB"}, "term_months": 12, "purpose": "PURPOSE_TAG_OTHER", "purpose_description": description})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := New(tx).Apply(ctx, receive.Input{EventID: uuid.New(), ApplicationID: id, AggregateVersion: 1, OrgID: uuid.New(), ClientUserID: uuid.New(), ProductID: productID, ProductVersion: 1, CompanyINN: "7707083893", CompanyName: "Test", CompanyJSON: []byte(`{"inn":"7707083893","name":"Test","registered_at":"2020-01-01","okved":"47.11","revenue_12m":{"amount_kopecks":10000,"currency":"RUB"},"profit_12m":{"amount_kopecks":1000,"currency":"RUB"},"obligations":{"amount_kopecks":0,"currency":"RUB"}}`), FormJSON: formJSON, DocumentsJSON: []byte(`[]`), AmountKopecks: 1000, TermMonths: 12, PurposeTag: "OTHER"})
	if err != nil || !applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	got, found, err := caserepository.New(tx).GetCase(ctx, id)
	if err != nil || !found || got.Application.Purpose != "OTHER" || got.Application.PurposeDescription != description {
		t.Fatalf("bank application=%+v found=%v err=%v", got.Application, found, err)
	}
}
