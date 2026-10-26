package assessment

import (
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/creditcalc"
)

const policyConfig = `{"version":"sme-cashflow-policy-v2","min_activity_months":12,"historical_dpd_reject":90,"base_coverage_bps":13000,"stress_coverage_bps":11000,"stress_receipts_bps":8000,"stress_variable_costs_bps":9000,"stress_receipt_delay_bps":2000,"cash_floor_kopecks":0,"amount_step_kopecks":100000,"max_candidates":3,"manager_confirmation":true}`

func validUntil(in Input) time.Time {
	t := in.AsOf.Add(24 * time.Hour)
	for _, s := range in.Sources.Checks {
		if s.ValidUntil.Before(t) {
			t = s.ValidUntil
		}
	}
	return t
}
func rate(p Product, grade string) int {
	if grade == "A" {
		return p.RateABPS
	}
	return p.RateBBPS
}

// CheckOffer is also the final manual-confirmation gate. Call with the original
// request, current product versions and the pinned assessment input.
func CheckOffer(in Input, productID string, t creditcalc.Terms) (Candidate, error) {
	if Checks(in).Conclusion != "CLEAR" {
		return Candidate{}, errors.New("ADMISSION_NOT_CLEAR")
	}
	f, issues := Normalize(in)
	if len(issues) > 0 {
		return Candidate{}, errors.New("FINANCIALS_INSUFFICIENT")
	}
	risk := Score(f, in.Sources.Credit)
	if risk.Status != "ASSESSED" || risk.Grade != "A" && risk.Grade != "B" {
		return Candidate{}, errors.New("RISK_NOT_ELIGIBLE")
	}
	return checkTerms(in, f, risk, productID, t)
}

func checkTerms(in Input, f FinancialSnapshot, risk RiskResult, productID string, t creditcalc.Terms) (Candidate, error) {
	var p Product
	found := false
	for _, v := range in.Products {
		if v.ID == productID {
			p = v
			found = true
			break
		}
	}
	if !found || !slices.Contains(p.Purposes, in.Purpose) {
		return Candidate{}, errors.New("PRODUCT_UNAVAILABLE")
	}
	if t.AmountKopecks > in.RequestedAmount || t.AmountKopecks < p.MinAmount || t.AmountKopecks > p.MaxAmount || t.TermMonths < p.MinTerm || t.TermMonths > p.MaxTerm {
		return Candidate{}, errors.New("PRODUCT_RANGE")
	}
	pref := in.Submission.Preferences
	minimum := in.RequestedAmount // Absence never implies consent to a smaller useful amount.
	if pref.MinimumUsefulAmountKopecks != nil {
		minimum = *pref.MinimumUsefulAmountKopecks
	}
	if t.AmountKopecks < minimum {
		return Candidate{}, errors.New("BELOW_USEFUL_AMOUNT")
	}
	if pref.MaxTermMonths != nil && t.TermMonths > *pref.MaxTermMonths {
		return Candidate{}, errors.New("PREFERRED_TERM_EXCEEDED")
	}
	if t.RateBPS != rate(p, risk.Grade) {
		return Candidate{}, errors.New("RATE_NOT_ALLOWED")
	}
	schedule, err := creditcalc.BuildSchedule(t, in.AsOf)
	if err != nil {
		return Candidate{}, err
	}
	if pref.MaxPaymentKopecks != nil {
		for _, p := range schedule.Payments {
			if p.PaymentKopecks > *pref.MaxPaymentKopecks {
				return Candidate{}, errors.New("PREFERRED_PAYMENT_EXCEEDED")
			}
		}
	}
	base, bcr, err := checkCash(f.Base, schedule, f.InitialCashKopecks, 13000)
	if err != nil {
		return Candidate{}, errors.New("BASE_" + err.Error())
	}
	stress, scr, err := checkCash(f.Stress, schedule, f.InitialCashKopecks, 11000)
	if err != nil {
		return Candidate{}, errors.New("STRESS_" + err.Error())
	}
	c := Candidate{ProductID: p.ID, ProductVersion: p.Version, ProductName: p.Name, Schedule: schedule, UncoveredNeedKopecks: in.RequestedAmount - t.AmountKopecks, Base: base, Stress: stress, BaseCoverageBPS: bcr, StressCoverageBPS: scr, Status: "CALCULATED", ValidUntil: validUntil(in)}
	c.ID = hash(struct {
		InputHash string
		Product   Product
		Terms     creditcalc.Terms
	}{hash(in), p, t})
	return c, nil
}

func checkCash(rows []CashMonth, s creditcalc.Schedule, reserve int64, required int) ([]CashMonth, int, error) {
	if len(rows) < s.TermMonths {
		return nil, 0, errors.New("FORECAST_TOO_SHORT")
	}
	out := append([]CashMonth(nil), rows[:s.TermMonths]...)
	cash := reserve
	minimumCoverage := int(^uint(0) >> 1)
	for i := range out {
		out[i].NewDebtKopecks = s.Payments[i].PaymentKopecks
		cash += out[i].CFADSKopecks - out[i].ExistingDebtKopecks - out[i].NewDebtKopecks
		out[i].ClosingCashKopecks = cash
		if cash < 0 {
			return nil, 0, errors.New("CASH_GAP")
		}
	}
	// Every rolling 12-month window (or the complete shorter term) must pass.
	window := min(12, len(out))
	for start := 0; start+window <= len(out); start++ {
		var cf, debt int64
		for _, m := range out[start : start+window] {
			cf += m.CFADSKopecks
			debt += m.ExistingDebtKopecks + m.NewDebtKopecks
		}
		coverage := ratio(cf, debt)
		minimumCoverage = min(minimumCoverage, coverage)
		if coverage < required {
			return nil, coverage, errors.New("COVERAGE_BELOW_MINIMUM")
		}
	}
	return out, minimumCoverage, nil
}

func Generate(in Input, f FinancialSnapshot, risk RiskResult) []Candidate {
	options := []Candidate{}
	seen := map[string]bool{}
	if risk.Status != "ASSESSED" || risk.Grade != "A" && risk.Grade != "B" {
		return options
	}
	minimum := in.RequestedAmount
	if in.Submission.Preferences.MinimumUsefulAmountKopecks != nil {
		minimum = *in.Submission.Preferences.MinimumUsefulAmountKopecks
	}
	for _, p := range in.Products {
		if !slices.Contains(p.Purposes, in.Purpose) {
			continue
		}
		if p.MinTerm < 1 || p.MaxTerm > 120 || p.MaxTerm < p.MinTerm || p.RateABPS < 0 || p.RateBBPS < 0 || p.RateABPS > 10000 || p.RateBBPS > 10000 {
			continue
		}
		maxTerm := p.MaxTerm
		if in.Submission.Preferences.MaxTermMonths != nil {
			maxTerm = min(maxTerm, *in.Submission.Preferences.MaxTermMonths)
		}
		for term := p.MinTerm; term <= maxTerm; term++ {
			upper := min(in.RequestedAmount, p.MaxAmount)
			t := creditcalc.Terms{AmountKopecks: upper, TermMonths: term, RateBPS: rate(p, risk.Grade)}
			c, err := checkTerms(in, f, risk, p.ID, t)
			if err != nil && minimum < upper {
				const step int64 = 100000 // 1,000 RUB; always round principal down.
				lo := (max(minimum, p.MinAmount) + step - 1) / step
				hi := upper / step
				best := int64(0)
				for lo <= hi {
					mid := (lo + hi) / 2
					t.AmountKopecks = mid * step
					_, e := checkTerms(in, f, risk, p.ID, t)
					if e == nil {
						best = mid
						lo = mid + 1
					} else {
						hi = mid - 1
					}
				}
				if best > 0 {
					t.AmountKopecks = best * step
					c, err = checkTerms(in, f, risk, p.ID, t)
				}
			}
			if err == nil && !seen[c.ID] {
				seen[c.ID] = true
				options = append(options, c)
			}
		}
	}
	priority := in.Submission.Preferences.Priority
	sort.Slice(options, func(i, j int) bool {
		a, b := options[i], options[j]
		if priority == "LOW_PAYMENT" && a.Schedule.PaymentKopecks != b.Schedule.PaymentKopecks {
			return a.Schedule.PaymentKopecks < b.Schedule.PaymentKopecks
		}
		if priority == "LOW_TOTAL_COST" && a.Schedule.TotalPaymentKopecks != b.Schedule.TotalPaymentKopecks {
			return a.Schedule.TotalPaymentKopecks < b.Schedule.TotalPaymentKopecks
		}
		if a.UncoveredNeedKopecks != b.UncoveredNeedKopecks {
			return a.UncoveredNeedKopecks < b.UncoveredNeedKopecks
		}
		if a.Schedule.TotalInterestKopecks != b.Schedule.TotalInterestKopecks {
			return a.Schedule.TotalInterestKopecks < b.Schedule.TotalInterestKopecks
		}
		return a.ID < b.ID
	})
	if len(options) > 3 {
		options = options[:3]
	}
	// Retain the requested terms when affordable, so the UI can compare them
	// with optimized alternatives without reconstructing a calculation.
	for _, p := range in.Products {
		if p.ID == in.RequestedProductID {
			original, err := checkTerms(in, f, risk, p.ID, creditcalc.Terms{AmountKopecks: in.RequestedAmount, TermMonths: in.RequestedTerm, RateBPS: rate(p, risk.Grade)})
			if err == nil {
				present := false
				for _, c := range options {
					present = present || c.ID == original.ID
				}
				if !present {
					if len(options) == 3 {
						options[2] = original
					} else {
						options = append(options, original)
					}
				}
			}
			break
		}
	}
	return options
}
