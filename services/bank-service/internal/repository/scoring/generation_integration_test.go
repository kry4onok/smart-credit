package scoringrepository

import (
	"context"
	"os"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLateAttemptCannotUpdateNewAssessmentGeneration(t *testing.T) {
	url := os.Getenv("BANK_DATABASE_URL")
	if url == "" {
		t.Skip("BANK_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, attempt, product := uuid.New(), uuid.New(), uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO products(id,version,code,name,purpose_tags,required_document_types,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months,indicative_rate_bps,is_active) VALUES($1,1,$2,'Test',ARRAY['WORKING_CAPITAL'],'{}',100,100000000,1,36,1000,true)`, product, product.String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO cases(id,org_id,client_user_id,product_id,product_version,product_name,company_inn,company_name,company_snapshot,application_snapshot,documents_snapshot,requested_amount_kopecks,requested_term_months,purpose_tag,status,client_event_version,assessment_generation) VALUES($1,$2,$3,$4,1,'Test','7707083893','Test','{}','{}','[]',1000000,12,'WORKING_CAPITAL','SCORING',1,2)`, id, uuid.New(), uuid.New(), product)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO scoring_attempts(id,case_id,input_revision,as_of_date,request,assessment_generation) VALUES($1,$2,1,current_date,'{}',1)`, attempt, id)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately retain the old pointer: generation guard alone must reject it.
	_, err = tx.Exec(ctx, `UPDATE cases SET current_scoring_attempt_id=$2 WHERE id=$1`, id, attempt)
	if err != nil {
		t.Fatal(err)
	}
	repo := New(tx, transaction.New(tx))
	applied, err := repo.Complete(ctx, attempt, nil, "SCORING_UNAVAILABLE")
	if err != nil || applied {
		t.Fatalf("late completion: %v %v", applied, err)
	}
	var version, generation int64
	var status string
	if err = tx.QueryRow(ctx, `SELECT version,assessment_generation,status FROM cases WHERE id=$1`, id).Scan(&version, &generation, &status); err != nil {
		t.Fatal(err)
	}
	if version != 1 || generation != 2 || status != "SCORING" {
		t.Fatalf("stale result mutated case: %d %d %s", version, generation, status)
	}
}
