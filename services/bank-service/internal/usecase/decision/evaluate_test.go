package decision

import (
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
)

func TestEvaluateRuleOrderAndOffer(t *testing.T) {
	now := time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)
	base := Input{
		Snapshot: validation.Snapshot{
			CompanyJSON:   []byte(`{"inn":"7707083893","name":"Test","registered_at":"2019-04-15","revenue_12m":{"amount_kopecks":840000000,"currency":"RUB"},"profit_12m":{"amount_kopecks":100000000,"currency":"RUB"},"obligations":{"amount_kopecks":10000000,"currency":"RUB"}}`),
			DocumentsJSON: []byte(`[{"document_id":"40000000-0000-4000-8000-000000000001","document_type":"BANK_STATEMENT","file_name":"statement.pdf","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]`),
			AmountKopecks: 300000000, TermMonths: 18, PurposeTag: "WORKING_CAPITAL",
		},
		Product: Product{Found: true, Purposes: []string{"WORKING_CAPITAL"}, Documents: []string{"BANK_STATEMENT"},
			MinAmount: 100000000, MaxAmount: 5000000000, MinTerm: 6, MaxTerm: 36,
			MinAgeMonths: 12, MaxAmountRevenueBP: 5000, MaxOverdueDays: 30,
			GradeARateBP: 1500, GradeBRateBP: 1600, OfferTTLHours: 72},
		Grade: "B", ScoreFound: true, CompanyStatus: "COMPANY_STATUS_ACTIVE", CompanyOK: true,
		CreditOK: true, Now: now,
	}
	tests := []struct {
		name   string
		change func(*Input)
		status model.CaseStatus
		rule   string
		rate   int
	}{
		{"offer", func(*Input) {}, model.CaseStatusOffer, "AUTO_OFFER", 1600},
		{"missing document before stop factor", func(in *Input) {
			in.Snapshot.DocumentsJSON = []byte(`[]`)
			in.OverdueDays = 100
		}, model.CaseStatusNeedsInfo, "MISSING_REQUIRED_DATA", 0},
		{"liquidating before grade", func(in *Input) {
			in.CompanyStatus = "LIQUIDATING"
			in.Grade = "C"
		}, model.CaseStatusRejected, "COMPANY_LIQUIDATION", 0},
		{"overdue stop factor", func(in *Input) { in.OverdueDays = 90 }, model.CaseStatusRejected, "OVERDUE_90_DAYS", 0},
		{"grade E", func(in *Input) { in.Grade = "E" }, model.CaseStatusRejected, "GRADE_E", 0},
		{"grade C", func(in *Input) { in.Grade = "C" }, model.CaseStatusManualReview, "GRADE_REVIEW", 0},
		{"amount exceeds revenue", func(in *Input) { in.Snapshot.AmountKopecks = 500000000 }, model.CaseStatusManualReview, "AMOUNT_REVENUE_REVIEW", 0},
		{"product overdue", func(in *Input) { in.OverdueDays = 31 }, model.CaseStatusManualReview, "OVERDUE_PRODUCT_REVIEW", 0},
		{"bureau missing", func(in *Input) { in.CreditOK = false }, model.CaseStatusManualReview, "BUREAU_RESULT_UNAVAILABLE", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.change(&in)
			got := Evaluate(in)
			if got.Status != tt.status || len(got.Rules) != 1 || got.Rules[0] != tt.rule || got.RateBP != tt.rate {
				t.Fatalf("Evaluate() = %+v", got)
			}
		})
	}
}
