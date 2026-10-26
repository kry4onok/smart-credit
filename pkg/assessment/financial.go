package assessment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"time"
)

// Keep sums and serialized money within JavaScript's exact integer range.
const maxComponent int64 = 1_000_000_000_000

func hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func issue(code, path, message string) Issue { return Issue{code, path, message} }
func ratio(n, d int64) int {
	if d <= 0 {
		return 0
	}
	return int(new(big.Int).Quo(new(big.Int).Mul(big.NewInt(n), big.NewInt(10000)), big.NewInt(d)).Int64())
}
func scale(n int64, bps int) int64 {
	return new(big.Int).Quo(new(big.Int).Mul(big.NewInt(n), big.NewInt(int64(bps))), big.NewInt(10000)).Int64()
}
func periodStart(asOf time.Time) time.Time {
	return time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -12, 0)
}
func validProvenance(p Provenance, asOf time.Time) bool {
	return p.SourceRef != "" && !p.ObservedAt.IsZero() && !p.ObservedAt.After(asOf.Add(24*time.Hour)) && (p.Quality == "DECLARED" || p.Quality == "PROVIDED" || p.Quality == "VERIFIED")
}
func comparable(f *FinancialInput) *FinancialInput {
	if f == nil {
		return nil
	}
	v := *f
	v.Provenance = Provenance{}
	v.Months = append([]Month(nil), f.Months...)
	for i := range v.Months {
		v.Months[i].Provenance = Provenance{}
	}
	sort.Slice(v.Months, func(i, j int) bool { return v.Months[i].Period < v.Months[j].Period })
	if v.BalanceSheet != nil {
		b := *v.BalanceSheet
		b.Provenance = Provenance{}
		v.BalanceSheet = &b
	}
	v.DebtPayments = append([]DebtPayment(nil), f.DebtPayments...)
	sort.Slice(v.DebtPayments, func(i, j int) bool {
		return v.DebtPayments[i].ObligationID+v.DebtPayments[i].Period < v.DebtPayments[j].ObligationID+v.DebtPayments[j].Period
	})
	return &v
}

// Normalize never substitutes accounting profit for cash flow. The monthly input
// is already classified operating cash (net of refunds); loans, own transfers and
// capital contributions must not be included in operating_receipts_kopecks.
func Normalize(in Input) (FinancialSnapshot, []Issue) {
	issues := []Issue{}
	f := in.Sources.Financials
	if declared := in.Submission.Financials; declared != nil {
		if f != nil && !reflect.DeepEqual(comparable(f), comparable(declared)) {
			return FinancialSnapshot{}, []Issue{issue("SOURCE_CONFLICT", "financials", "Заявленные и полученные финансовые данные различаются")}
		}
		if f == nil {
			f = declared
		}
	}
	if f == nil {
		return FinancialSnapshot{}, []Issue{issue("MISSING_FINANCIALS", "financials", "Передайте помесячные денежные потоки и график действующих долгов")}
	}
	if f.Currency != "RUB" || !validProvenance(f.Provenance, in.AsOf) {
		issues = append(issues, issue("INVALID_FINANCIAL_SOURCE", "financials.provenance", "Нужны RUB и происхождение данных"))
	}
	if f.InitialCashKopecks == nil || *f.InitialCashKopecks < 0 || *f.InitialCashKopecks > maxComponent {
		issues = append(issues, issue("MISSING_OR_INVALID_VALUE", "financials.initial_cash_kopecks", "Укажите доступный денежный резерв"))
	}
	if f.DebtScheduleKnown == nil || !*f.DebtScheduleKnown {
		issues = append(issues, issue("UNKNOWN_DEBT_SCHEDULE", "financials.debt_schedule_known", "Неизвестный график долга не равен отсутствию долга"))
	}
	horizon := in.RequestedTerm
	for _, p := range in.Products {
		horizon = max(horizon, p.MaxTerm)
	}
	horizon = min(horizon, 120)
	if in.Submission.Preferences.MaxTermMonths != nil {
		horizon = min(horizon, *in.Submission.Preferences.MaxTermMonths)
	}
	month := time.Date(in.AsOf.Year(), in.AsOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	through, err := time.Parse("2006-01", f.DebtScheduleThrough)
	if err != nil || through.Before(month.AddDate(0, horizon, 0)) {
		issues = append(issues, issue("INCOMPLETE_DEBT_SCHEDULE", "financials.debt_schedule_through", "Нужен график действующих долгов на весь горизонт расчёта, включая остаточный платёж"))
	}
	start := periodStart(in.AsOf)
	months := append([]Month(nil), f.Months...)
	sort.Slice(months, func(i, j int) bool { return months[i].Period < months[j].Period })
	if len(months) != 12 {
		issues = append(issues, issue("INCOMPLETE_HISTORY", "financials.months", "Нужны последние 12 полных календарных месяцев"))
	}
	for i, m := range months {
		if i >= 12 || m.Period != start.AddDate(0, i, 0).Format("2006-01") {
			issues = append(issues, issue("INVALID_OR_DUPLICATE_PERIOD", fmt.Sprintf("financials.months[%d]", i), "Периоды должны быть последовательными и не повторяться"))
		}
		for name, v := range map[string]*int64{"receipts": m.Receipts, "variable_costs": m.VariableCosts, "fixed_costs": m.FixedCosts, "taxes": m.Taxes, "maintenance": m.Maintenance, "owner_drawings": m.OwnerDrawings} {
			if v == nil || *v < 0 || *v > maxComponent {
				issues = append(issues, issue("MISSING_OR_INVALID_VALUE", fmt.Sprintf("financials.months[%d].%s", i, name), "Пропуск не равен нулю; нужны неотрицательные денежные значения"))
			}
		}
		if !validProvenance(m.Provenance, in.AsOf) {
			issues = append(issues, issue("INVALID_PROVENANCE", fmt.Sprintf("financials.months[%d]", i), "Не указано происхождение месячных данных"))
		}
	}
	if len(f.DebtPayments) > 1200 {
		issues = append(issues, issue("INPUT_TOO_LARGE", "financials.debt_payments", "Слишком большой график долгов"))
	}
	debts := map[string]int64{}
	seen := map[string]bool{}
	for _, d := range f.DebtPayments {
		key := d.ObligationID + ":" + d.Period
		period, e := time.Parse("2006-01", d.Period)
		if d.ObligationID == "" || seen[key] || e != nil || d.AmountKopecks < 0 || d.AmountKopecks > maxComponent || e == nil && (period.Before(month.AddDate(0, 1, 0)) || period.After(through)) {
			issues = append(issues, issue("INVALID_DEBT_PAYMENT", "financials.debt_payments", "Проверьте даты, сумму и уникальность платежей по каждому долгу"))
		}
		seen[key] = true
		debts[d.Period] += d.AmountKopecks
	}
	if in.Sources.Credit.TotalDebtKopecks != nil && *in.Sources.Credit.TotalDebtKopecks > 0 && len(f.DebtPayments) == 0 {
		issues = append(issues, issue("DEBT_CONFLICT", "financials.debt_payments", "БКИ содержит долг, но платежи не переданы"))
	}
	if b := f.BalanceSheet; b != nil {
		date, e := time.Parse(time.DateOnly, b.AsOf)
		if in.LegalForm != "LLC" || e != nil || date.After(in.AsOf) || b.AssetsKopecks < 0 || b.LiabilitiesKopecks < 0 || b.AssetsKopecks > maxComponent || b.LiabilitiesKopecks > maxComponent || b.ReceivablesKopecks < 0 || b.PayablesKopecks < 0 || b.ReceivablesKopecks > b.AssetsKopecks || b.PayablesKopecks > b.LiabilitiesKopecks || b.EquityKopecks != b.AssetsKopecks-b.LiabilitiesKopecks || !validProvenance(b.Provenance, in.AsOf) {
			issues = append(issues, issue("INVALID_BALANCE", "financials.balance_sheet", "Баланс не согласован по дате, форме или арифметике"))
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Path+issues[i].Code < issues[j].Path+issues[j].Code })
	if len(issues) > 0 {
		return FinancialSnapshot{}, issues
	}
	s := FinancialSnapshot{Currency: "RUB", Unit: "KOPECKS", PeriodFrom: start.Format(time.DateOnly), PeriodTo: month.AddDate(0, 0, -1).Format(time.DateOnly), ForecastMethod: "repeat-last-12-full-months-no-growth-v1", InitialCashKopecks: *f.InitialCashKopecks, LargestCustomerShareBPS: f.LargestCustomerShareBPS, Provenance: f.Provenance, BalanceStatus: "NOT_PROVIDED", History: []CashMonth{}, Base: []CashMonth{}, Stress: []CashMonth{}}
	if in.LegalForm == "IP" {
		s.BalanceStatus = "NOT_APPLICABLE"
	} else if f.BalanceSheet != nil {
		s.BalanceStatus = f.BalanceSheet.Provenance.Quality
	}
	var receipts, total int64
	for _, m := range months {
		cf := *m.Receipts - *m.VariableCosts - *m.FixedCosts - *m.Taxes - *m.Maintenance - *m.OwnerDrawings
		receipts += *m.Receipts
		total += cf
		s.History = append(s.History, CashMonth{Period: m.Period, CFADSKopecks: cf, ReceiptsKopecks: *m.Receipts, Quality: m.Provenance.Quality, EvidenceRefs: []string{m.Provenance.SourceRef}})
	}
	if receipts <= 0 {
		return FinancialSnapshot{}, []Issue{issue("ZERO_OPERATING_RECEIPTS", "financials.months", "Нет операционных поступлений для оценки устойчивого денежного потока")}
	}
	s.MarginBPS = ratio(total, receipts)
	var delayed int64
	for i := 1; i <= horizon; i++ {
		future := month.AddDate(0, i, 0)
		var m Month
		for _, candidate := range months {
			d, _ := time.Parse("2006-01", candidate.Period)
			if d.Month() == future.Month() {
				m = candidate
				break
			}
		}
		base := *m.Receipts - *m.VariableCosts - *m.FixedCosts - *m.Taxes - *m.Maintenance - *m.OwnerDrawings
		// Stress: receipts -20%, variable costs -10%; 20% of stressed
		// receipts arrive one month late. No opening delayed receivables assumed.
		gross := scale(*m.Receipts, 8000)
		nextDelayed := scale(gross, 2000)
		stressedReceipts := gross - nextDelayed + delayed
		delayed = nextDelayed
		stress := stressedReceipts - scale(*m.VariableCosts, 9000) - *m.FixedCosts - *m.Taxes - *m.Maintenance - *m.OwnerDrawings
		period := future.Format("2006-01")
		row := CashMonth{Period: period, CFADSKopecks: base, ReceiptsKopecks: *m.Receipts, ExistingDebtKopecks: debts[period], Quality: "PROJECTED", EvidenceRefs: []string{m.Provenance.SourceRef, f.Provenance.SourceRef}}
		s.Base = append(s.Base, row)
		row.CFADSKopecks = stress
		row.ReceiptsKopecks = stressedReceipts
		s.Stress = append(s.Stress, row)
	}
	s.ID = hash(s)
	return s, issues
}
