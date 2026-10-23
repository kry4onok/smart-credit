package receiverepository

import (
	"context"
	"os"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
