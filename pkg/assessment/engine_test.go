package assessment

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/creditcalc"
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

func TestNotRunAndExtremeRatiosAreExplicit(t *testing.T) {
	in := demoInput()
	in.Sources.CompanyStatus = "LIQUIDATING"
	r := Evaluate(in)
	if r.Risk.Status != "NOT_RUN" || r.Risk.Score != nil || r.Risk.ModelVersion != ModelVersion || r.Risk.ConfigChecksum == "" {
		t.Fatal(r.Risk)
	}
	if got := ratio(-72_000_000_000_000, 1); got != -1_000_000_000 {
		t.Fatal(got)
	}
}

func TestScoreFactorThresholdsAndMissing(t *testing.T) {
	in := demoInput()
	f, issues := Normalize(in)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	for _, tc := range []struct{ dpd, want int }{{0, 400}, {1, 320}, {7, 320}, {8, 200}, {29, 200}, {30, 80}, {59, 80}, {60, 0}, {89, 0}} {
		in.Sources.Credit.HistoricalMaxDPD = ptr(tc.dpd)
		r := Score(f, in.Sources.Credit)
		if r.Factors[0].Points != tc.want {
			t.Fatalf("dpd %d: %v", tc.dpd, r)
		}
	}
	for _, tc := range []struct{ bps, want int }{{-1, 0}, {0, 35}, {499, 35}, {500, 105}, {999, 105}, {1000, 210}, {1499, 210}, {1500, 280}, {1999, 280}, {2000, 350}} {
		f.MarginBPS = tc.bps
		r := Score(f, in.Sources.Credit)
		if r.Factors[1].Points != tc.want {
			t.Fatalf("margin %d: %v", tc.bps, r)
		}
	}
	for _, tc := range []struct{ bps, want int }{{0, 250}, {2000, 250}, {2001, 200}, {3500, 200}, {3501, 125}, {5000, 125}, {5001, 50}, {7500, 50}, {7501, 0}, {10000, 0}} {
		f.LargestCustomerShareBPS = ptr(tc.bps)
		r := Score(f, in.Sources.Credit)
		if r.Factors[2].Points != tc.want {
			t.Fatalf("share %d: %v", tc.bps, r)
		}
	}
	f.LargestCustomerShareBPS = nil
	r := Score(f, in.Sources.Credit)
	if r.Score != nil || r.Status != "INSUFFICIENT_DATA" {
		t.Fatal(r)
	}
}

func TestRelatedNegativeUnavailableAndConflictNeverBecomeBorrowerRejection(t *testing.T) {
	for _, change := range []func(*Input){
		func(in *Input) { in.Sources.Checks[5].Status = "SUCCESS"; in.Sources.Checks[5].Conclusion = "BLOCK" },
		func(in *Input) { in.Sources.Checks[5].Status = "AMBIGUOUS"; in.Sources.Checks[5].Conclusion = "BLOCK" },
		func(in *Input) { in.Sources.Checks[0].Status = "UNAVAILABLE"; in.Sources.RegisteredAt = "2026-10-25" },
		func(in *Input) {
			in.Sources.Credit.HistoricalMaxDPD = ptr(90)
			in.Sources.Credit.OverdueEpisodes = ptr(0)
		},
	} {
		in := demoInput()
		change(&in)
		r := Evaluate(in)
		if r.Status != "MANUAL_REVIEW" || r.Risk.Score != nil || len(r.Candidates) != 0 {
			t.Fatal(r)
		}
	}
}

func TestDemoDebtScheduleOnLateMonthDays(t *testing.T) {
	for _, day := range []string{"2026-10-29", "2026-10-31", "2027-01-30", "2026-08-31"} {
		asOf, _ := time.Parse(time.DateOnly, day)
		start := periodStart(asOf)
		c := CreditHistory{HistoricalMaxDPD: ptr(0), CurrentDPD: ptr(0), OverdueEpisodes: ptr(0), TotalDebtKopecks: ptr(int64(120_000_000)), PeriodFrom: start.Format(time.DateOnly), PeriodTo: start.AddDate(1, 0, -1).Format(time.DateOnly)}
		in := demoInput()
		in.AsOf = asOf
		in.Sources = DemoSources("7707083893", "2020-01-01", "ACTIVE", c, asOf)
		seen := map[string]bool{}
		for _, p := range in.Sources.Financials.DebtPayments {
			if seen[p.Period] {
				t.Fatalf("%s: two demo debt payments in %s", day, p.Period)
			}
			seen[p.Period] = true
		}
		for _, issue := range Evaluate(in).Issues {
			if issue.Code == "INVALID_DEBT_PAYMENT" {
				t.Fatalf("%s: demo schedule rejected: %+v", day, issue)
			}
		}
	}
}
