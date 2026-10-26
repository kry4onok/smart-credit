package product

import (
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/testutil"
	"github.com/google/uuid"
)

func TestPurposeRanksWithoutHidingProductsIntegration(t *testing.T) {
	db := testutil.Postgres(t)
	for _, p := range []struct{ code, purpose string }{{"A", "WORKING_CAPITAL"}, {"B", "EQUIPMENT"}, {"C", "OTHER"}} {
		if _, err := db.Exec(t.Context(), `INSERT INTO products(id,version,code,name,purpose_tags,min_amount_kopecks,max_amount_kopecks,min_term_months,max_term_months) VALUES($1,1,$2,$2,ARRAY[$3]::text[],100,10000,1,12)`, uuid.New(), p.code, p.purpose); err != nil {
			t.Fatal(err)
		}
	}
	for _, purpose := range []string{"EQUIPMENT", "OTHER", "EXPANSION", ""} {
		var preferred *string
		if purpose != "" {
			preferred = &purpose
		}
		got, err := New(db).List(t.Context(), preferred)
		if err != nil || len(got.Items) != 3 {
			t.Fatalf("purpose=%s items=%v error=%v", purpose, got.Items, err)
		}
		want := "A"
		if purpose == "EQUIPMENT" {
			want = "B"
		}
		if purpose == "OTHER" {
			want = "C"
		}
		if got.Items[0].Code != want {
			t.Fatalf("first=%s want=%s", got.Items[0].Code, want)
		}
	}
}
