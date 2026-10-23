package validationrepository

import (
	"context"
	"errors"
	"os"
	"testing"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	caserepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/case"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
)

func TestCompletePersistsValidationAndPublicRequest(t *testing.T) {
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
INSERT INTO products (id, version, code, name, purpose_tags, required_document_types,
    min_amount_kopecks, max_amount_kopecks, min_term_months, max_term_months,
    indicative_rate_bps)
VALUES ($1, 1, $2, 'Validation test', ARRAY['WORKING_CAPITAL'],
    ARRAY['BANK_STATEMENT'], 1, 1000000, 1, 36, 1000)`,
		productID, "VALIDATION-TEST-"+productID.String())
	if err != nil {
		t.Fatal(err)
	}
	caseID := uuid.New()
	_, err = tx.Exec(ctx, `
INSERT INTO cases (id, org_id, client_user_id, product_id, product_version, product_name,
    company_inn, company_name, company_snapshot, application_snapshot, documents_snapshot,
    requested_amount_kopecks, requested_term_months, purpose_tag, status,
    next_run_at, client_event_version)
VALUES ($1, $2, $3, $4, 1, 'Validation test', '7707083893', 'Test',
    '{"inn":"7707083893","name":"Test","okved":"10.71","registered_at":"2020-01-01"}'::jsonb,
    '{}'::jsonb, '[]'::jsonb, 1000, 12, 'WORKING_CAPITAL', 'VALIDATION', now(), 1)`,
		caseID, uuid.New(), uuid.New(), productID)
	if err != nil {
		t.Fatal(err)
	}
	claim := validation.Claim{CaseID: caseID, Version: 1, ProductID: productID, ProductVersion: 1}
	product, err := repository.LoadProduct(ctx, claim)
	if err != nil || !product.Found || len(product.RequiredDocumentTypes) != 1 {
		t.Fatalf("LoadProduct = %+v, %v", product, err)
	}
	outcome := validation.Outcome{
		Status: model.CaseStatusNeedsInfo, Reason: "MISSING_REQUIRED_DATA",
		RequiredFields:        []string{"COMPANY_REVENUE_12M"},
		RequiredDocumentTypes: []string{"BANK_STATEMENT"},
	}
	if err := repository.Complete(ctx, claim, outcome); err != nil {
		t.Fatal(err)
	}
	var status, recommendation, reason string
	var version, auditVersion int64
	var scheduled bool
	if err := tx.QueryRow(ctx, `SELECT status, recommendation, public_reason, version,
    audit_version, next_run_at IS NOT NULL FROM cases WHERE id=$1`, caseID).
		Scan(&status, &recommendation, &reason, &version, &auditVersion, &scheduled); err != nil {
		t.Fatal(err)
	}
	if status != "NEEDS_INFO" || recommendation != "REQUEST_INFO" || reason == "" ||
		version != 2 || auditVersion != 1 || scheduled {
		t.Fatalf("case = status %s, recommendation %s, version %d, audit %d, scheduled %v",
			status, recommendation, version, auditVersion, scheduled)
	}
	var requestID uuid.UUID
	var fields, documents []string
	if err := tx.QueryRow(ctx, `SELECT id, required_fields, required_document_types
    FROM info_requests WHERE case_id=$1 AND status='OPEN'`, caseID).
		Scan(&requestID, &fields, &documents); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0] != "COMPANY_REVENUE_12M" ||
		len(documents) != 1 || documents[0] != "BANK_STATEMENT" {
		t.Fatalf("info request = %v, %v", fields, documents)
	}
	var payload []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM outbox WHERE aggregate_id=$1 AND aggregate_version=2`,
		caseID.String()).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var event eventspb.Event
	if err := proto.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	public := event.GetApplicationPublicStatePublished()
	if public.GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO ||
		public.GetActiveInfoRequest().GetRequestId() != requestID.String() {
		t.Fatalf("public event = %v", public)
	}
	var auditCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE case_id=$1 AND case_version=2
    AND action='VALIDATION_COMPLETED'`, caseID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("audit rows = %d", auditCount)
	}
	caseItem, found, err := caserepository.New(tx).GetCase(ctx, caseID)
	if err != nil || !found || caseItem.InfoRequest == nil ||
		len(caseItem.InfoRequest.RequiredFields) != 1 {
		t.Fatalf("GetCase with missing money = %+v, found %v, error %v", caseItem, found, err)
	}
	if err := repository.Complete(ctx, claim, outcome); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("replayed Complete error = %v, want no rows", err)
	}

	readyCaseID := uuid.New()
	_, err = tx.Exec(ctx, `
INSERT INTO cases (id, org_id, client_user_id, product_id, product_version, product_name,
    company_inn, company_name, company_snapshot, application_snapshot, documents_snapshot,
    requested_amount_kopecks, requested_term_months, purpose_tag, status,
    next_run_at, client_event_version)
VALUES ($1, $2, $3, $4, 1, 'Validation test', '7707083893', 'Test',
    '{"inn":"7707083893","name":"Test","okved":"10.71","registered_at":"2020-01-01"}'::jsonb,
    '{}'::jsonb, '[]'::jsonb, 1000, 12, 'WORKING_CAPITAL', 'VALIDATION', now(), 1)`,
		readyCaseID, uuid.New(), uuid.New(), productID)
	if err != nil {
		t.Fatal(err)
	}
	readyClaim := validation.Claim{CaseID: readyCaseID, Version: 1, ProductID: productID, ProductVersion: 1}
	if err := repository.Complete(ctx, readyClaim, validation.Outcome{Status: model.CaseStatusEnrichment}); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT status, next_run_at IS NOT NULL FROM cases WHERE id=$1`, readyCaseID).
		Scan(&status, &scheduled); err != nil {
		t.Fatal(err)
	}
	if status != "ENRICHMENT" || !scheduled {
		t.Fatalf("ready case = status %s, scheduled %v", status, scheduled)
	}
}
