package assessment

import (
	"encoding/json"
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/creditcalc"
	"strings"
	"testing"
	"time"
)

func demoInput() Input {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	start := periodStart(now)
	c := CreditHistory{HistoricalMaxDPD: ptr(0), CurrentDPD: ptr(0), OverdueEpisodes: ptr(0), TotalDebtKopecks: ptr(int64(0)), PeriodFrom: start.Format(time.DateOnly), PeriodTo: start.AddDate(1, 0, -1).Format(time.DateOnly)}
	return Input{ApplicationID: "00000000-0000-4000-8000-000000000001", InputRevision: 1, Generation: 1, AsOf: now, INN: "7707083893", LegalForm: "LLC", RequestedProductID: "demo-product", RequestedAmount: 100_000_000, RequestedTerm: 12, Purpose: "WORKING_CAPITAL", Submission: Submission{SchemaVersion: 1, Preferences: Preferences{Priority: "KEEP_AMOUNT"}}, Sources: DemoSources("7707083893", "2020-01-01", "ACTIVE", c, now), Products: []Product{{ID: "demo-product", Version: 1, Name: "Демо", Purposes: []string{"WORKING_CAPITAL"}, MinAmount: 10_000_000, MaxAmount: 1_000_000_000, MinTerm: 6, MaxTerm: 36, RateABPS: 1800, RateBBPS: 2200}}}
}
func TestAssessmentCompleteAndPublicPrivacy(t *testing.T) {
	in := demoInput()
	r := Evaluate(in)
	if r.Status != "READY_FOR_CONFIRMATION" || r.Risk.Score == nil || len(r.Candidates) == 0 {
		t.Fatalf("%+v", r)
	}
	for _, c := range r.Candidates {
		if _, e := CheckOffer(in, c.ProductID, c.Schedule.Terms); e != nil {
			t.Fatal(e)
		}
		if c.Schedule.AmountKopecks > in.RequestedAmount {
			t.Fatal(c)
		}
	}
	data, _ := json.Marshal(Public(r))
	for _, word := range []string{"\"score\"", "\"checks\"", "historical_max_dpd", "risk."} {
		if strings.Contains(string(data), word) {
			t.Fatal(word)
		}
	}
	if r.Financial.Base[0].Quality != "PROJECTED" || r.Financial.PeriodFrom != "2025-10-01" || r.Financial.PeriodTo != "2026-09-30" {
		t.Fatal(r.Financial)
	}
}
func TestAdmissionBoundariesAndMissing(t *testing.T) {
	for _, dpd := range []int{89, 90, 91} {
		in := demoInput()
		in.Sources.Credit.HistoricalMaxDPD = ptr(dpd)
		in.Sources.Credit.OverdueEpisodes = ptr(1)
		got := Checks(in)
		if (got.Conclusion == "BLOCK") != (dpd >= 90) {
			t.Fatal(dpd, got)
		}
	}
	in := demoInput()
	in.Sources.RegisteredAt = "2025-10-27"
	if Checks(in).Conclusion != "BLOCK" {
		t.Fatal("young")
	}
	in.Sources.RegisteredAt = "2025-10-26"
	if Checks(in).Conclusion != "CLEAR" {
		t.Fatal("anniversary")
	}
	in = demoInput()
	in.Sources.Credit.CurrentDPD = nil
	if Checks(in).Conclusion != "REVIEW" {
		t.Fatal("missing treated as zero")
	}
	in = demoInput()
	in.Sources.Checks[2].Role = "DIRECTOR"
	in.Sources.Checks[2].SubjectID = "person"
	in.Sources.Checks[2].Match = "AMBIGUOUS"
	in.Sources.Checks[2].Conclusion = "BLOCK"
	if Checks(in).Conclusion == "BLOCK" {
		t.Fatal("ambiguous person became borrower block")
	}
}
func TestFinancialMissingConflictDuplicateAndDebt(t *testing.T) {
	in := demoInput()
	in.Sources.Financials.Months[0].Taxes = nil
	if r := Evaluate(in); r.Status != "NEEDS_INFO" || r.Risk.Score != nil {
		t.Fatal(r)
	}
	in = demoInput()
	in.Sources.Financials.Months[1].Period = in.Sources.Financials.Months[0].Period
	if _, issues := Normalize(in); len(issues) == 0 {
		t.Fatal("duplicate accepted")
	}
	in = demoInput()
	in.Sources.Credit.TotalDebtKopecks = ptr(int64(100))
	if _, issues := Normalize(in); len(issues) == 0 {
		t.Fatal("unknown debt payments accepted")
	}
	in = demoInput()
	f := *in.Sources.Financials
	f.Provenance.Quality = "PROVIDED"
	f.Months = append([]Month(nil), f.Months...)
	for i := range f.Months {
		f.Months[i].Provenance.Quality = "PROVIDED"
	}
	f.InitialCashKopecks = ptr(int64(123))
	in.Submission.Financials = &f
	if r := Evaluate(in); r.Status != "MANUAL_REVIEW" {
		t.Fatal(r.Status)
	}
}
func TestAlternativesAndManualGate(t *testing.T) {
	in := demoInput()
	in.RequestedAmount = 800_000_000
	in.RequestedTerm = 6
	in.Submission.Preferences.MinimumUsefulAmountKopecks = ptr(int64(10_000_000))
	r := Evaluate(in)
	if len(r.OriginalFailureCodes) == 0 || len(r.Candidates) == 0 {
		t.Fatal(r.Status, r.OriginalFailureCodes)
	}
	for _, c := range r.Candidates {
		if _, err := CheckOffer(in, c.ProductID, c.Schedule.Terms); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CheckOffer(in, "demo-product", creditcalc.Terms{AmountKopecks: in.RequestedAmount + 1, TermMonths: 36, RateBPS: 1800}); err == nil {
		t.Fatal("amount increased")
	}
	if _, err := CheckOffer(in, "demo-product", creditcalc.Terms{AmountKopecks: 10_000_000, TermMonths: 36, RateBPS: 1700}); err == nil {
		t.Fatal("rate reduced")
	}
	in.Submission.Preferences.MinimumUsefulAmountKopecks = nil
	r = Evaluate(in)
	for _, c := range r.Candidates {
		if c.Schedule.AmountKopecks < in.RequestedAmount {
			t.Fatal("smaller amount without consent")
		}
	}
}
func TestCashGapAndBalloon(t *testing.T) {
	in := demoInput()
	in.Sources.Financials.InitialCashKopecks = ptr(int64(0))
	in.Sources.Financials.Months[1].Receipts = ptr(int64(0))
	if r := Evaluate(in); len(r.Candidates) > 0 {
		t.Fatal("seasonal cash gap accepted")
	}
	in = demoInput()
	in.Sources.Financials.DebtPayments = []DebtPayment{{ObligationID: "balloon", Period: "2027-04", AmountKopecks: 10_000_000_000}}
	if r := Evaluate(in); len(r.Candidates) > 0 {
		t.Fatal("balloon ignored")
	}
}
func TestVersioningAndRiskIndependence(t *testing.T) {
	in := demoInput()
	a := Evaluate(in)
	in.RequestedAmount /= 2
	in.RequestedTerm = 24
	b := Evaluate(in)
	if *a.Risk.Score != *b.Risk.Score {
		t.Fatal("terms changed borrower risk")
	}
	in.Generation++
	if IsCurrent(b, in, in.AsOf) {
		t.Fatal("old generation accepted")
	}
	in = demoInput()
	if IsCurrent(Evaluate(in), in, in.AsOf.Add(25*time.Hour)) {
		t.Fatal("expired candidate accepted")
	}
	in.Sources.Financials.LargestCustomerShareBPS = nil
	if r := Evaluate(in); r.Risk.Score != nil || r.Status != "NEEDS_INFO" {
		t.Fatal(r)
	}
}
