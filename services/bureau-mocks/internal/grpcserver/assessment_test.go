package grpcserver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/demodata"
	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
)

// Exercise real catalogue -> bureau response -> financial normalization -> bank
// rules, rather than asserting that the scorecard repeats its own coefficients.
func TestDemoCompanyDecisionJourneys(t *testing.T) {
	service := testService(t)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	expected := map[string]struct {
		score         int
		grade, status string
	}{
		"client@smart-credit.test":    {1000, "A", "READY_FOR_CONFIRMATION"},
		"ip@smart-credit.test":        {950, "A", "READY_FOR_CONFIRMATION"}, // Patent is checked by the requirements engine.
		"closing@smart-credit.test":   {-1, "", "REJECT"},
		"warehouse@smart-credit.test": {750, "B", "READY_FOR_CONFIRMATION"},
		"workshop@smart-credit.test":  {950, "A", "READY_FOR_CONFIRMATION"},
		"cashgap@smart-credit.test":   {930, "A", "NO_AFFORDABLE_OPTION"},
		"loss@smart-credit.test":      {0, "E", "REJECT"},
		"review@smart-credit.test":    {605, "C", "MANUAL_REVIEW"},
		"service@smart-credit.test":   {-1, "", "MANUAL_REVIEW"},
	}
	for _, company := range demodata.Companies() {
		t.Run(company.Email, func(t *testing.T) {
			response, err := service.GetAssessmentSources(t.Context(), &bureaupb.AssessmentSourcesRequest{Inn: company.INN, AsOf: now.Format(time.RFC3339)})
			if err != nil {
				t.Fatal(err)
			}
			var sources assessment.Sources
			if err = json.Unmarshal(response.SourcesJson, &sources); err != nil {
				t.Fatal(err)
			}
			if sources.RegisteredAt != company.RegisteredAt || sources.Financials == nil || !sources.Financials.Provenance.IsMock {
				t.Fatal("inconsistent demo source")
			}
			minimum := int64(100000000)
			form := "LLC"
			if len(company.INN) == 12 {
				form = "IP"
			}
			in := assessment.Input{ApplicationID: company.INN, InputRevision: 1, Generation: 1, AsOf: now, INN: company.INN, LegalForm: form, RequestedProductID: "demo", RequestedAmount: company.SuggestedAmountKopecks, RequestedTerm: company.SuggestedTermMonths, Purpose: "WORKING_CAPITAL", Submission: assessment.Submission{SchemaVersion: 1, Preferences: assessment.Preferences{Priority: "KEEP_AMOUNT", MinimumUsefulAmountKopecks: &minimum}}, Sources: sources, Products: []assessment.Product{{ID: "demo", Version: 1, Name: "Demo", Purposes: []string{"WORKING_CAPITAL"}, MinAmount: 10000000, MaxAmount: 500000000, MinTerm: 6, MaxTerm: 36, RateABPS: 1600, RateBBPS: 1800}}}
			result := assessment.Evaluate(in)
			score := -1
			if result.Risk.Score != nil {
				score = *result.Risk.Score
			}
			want, ok := expected[company.Email]
			if !ok || score != want.score || result.Risk.Grade != want.grade || result.Status != want.status {
				t.Fatalf("score=%d grade=%s status=%s failures=%v; want %+v", score, result.Risk.Grade, result.Status, result.OriginalFailureCodes, want)
			}
			t.Logf("score=%d grade=%s status=%s candidates=%d", score, result.Risk.Grade, result.Status, len(result.Candidates))
			if result.Financial != nil && result.Financial.MarginBPS != company.MarginBPS {
				t.Fatal("wrong normalized margin")
			}
			for _, candidate := range result.Candidates {
				if _, err := assessment.CheckOffer(in, candidate.ProductID, candidate.Schedule.Terms); err != nil {
					t.Fatal(err)
				}
				wantRate := 1600
				if result.Risk.Grade == "B" {
					wantRate = 1800
				}
				if candidate.Schedule.RateBPS != wantRate {
					t.Fatal("wrong grade rate")
				}
			}
			if company.Email == "workshop@smart-credit.test" {
				if len(result.OriginalFailureCodes) == 0 || len(result.Candidates) == 0 {
					t.Fatal("counteroffer must fail original terms and provide alternatives")
				}
				for _, c := range result.Candidates {
					if c.Schedule.AmountKopecks == in.RequestedAmount && c.Schedule.TermMonths == in.RequestedTerm {
						t.Fatal("original unaffordable terms offered")
					}
				}
			}
			if sources.Credit.TotalDebtKopecks != nil && *sources.Credit.TotalDebtKopecks != company.DebtKopecks {
				t.Fatal("profile debt differs from bureau")
			}
		})
	}
}
