package caserepository

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	auditcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const auditTestCaseQuery = `INSERT INTO cases (id,org_id,client_user_id,product_id,product_version,product_name,company_inn,company_name,company_snapshot,application_snapshot,documents_snapshot,requested_amount_kopecks,requested_term_months,purpose_tag,status,client_event_version) VALUES ($1,$2,$3,$4,1,'Audit test','7707083893','Audit test','{}'::jsonb,'{}'::jsonb,'[]'::jsonb,100000000,12,'WORKING_CAPITAL','MANUAL_REVIEW',1)`
const auditTestEntryQuery = `INSERT INTO audit_log (id,case_id,case_version,action,actor_type,reason,details,occurred_at) VALUES ($1,$2,$3,$4,'SYSTEM',$5,$6::jsonb,$7)`

func TestListAuditOrdersTiesAndKeepsInternalDetailsPrivate(t *testing.T) {
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
	caseID := uuid.New()
	_, err = tx.Exec(ctx, auditTestCaseQuery, caseID, uuid.New(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for n, id := range ids {
		occurred := at
		if n == 2 {
			occurred = at.Add(time.Second)
		}
		_, err = tx.Exec(ctx, auditTestEntryQuery, id, caseID, n+1, "TEST", "reason", `{"from_status":"MANUAL_REVIEW","to_status":"OFFER","comment":"private note"}`, occurred)
		if err != nil {
			t.Fatal(err)
		}
	}
	repo := New(tx)
	exists, err := repo.CaseExists(ctx, caseID)
	if err != nil || !exists {
		t.Fatalf("exists=%v error=%v", exists, err)
	}
	first, err := repo.ListAudit(ctx, auditcase.Query{CaseID: caseID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0].ID != ids[0] || first[1].ID != ids[1] || first[0].FromStatus == nil || *first[0].FromStatus != "MANUAL_REVIEW" {
		t.Fatalf("first page=%+v", first)
	}
	next, err := repo.ListAudit(ctx, auditcase.Query{CaseID: caseID, After: &auditcase.Cursor{OccurredAt: first[1].OccurredAt, ID: first[1].ID}, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].ID != ids[2] {
		t.Fatalf("next page=%+v", next)
	}
}
