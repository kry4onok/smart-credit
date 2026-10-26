package assessment

import "time"

func ptr[T any](v T) *T { return &v }

// DemoSources creates synthetic cash data, never evidence of a real business.
// The caller must first resolve the INN in the explicit demonstration catalogue.
func DemoSources(inn, registered, status string, credit CreditHistory, asOf time.Time) Sources {
	p := Provenance{SourceRef: "demo-bank-ledger:" + inn, ObservedAt: asOf, Quality: "VERIFIED", IsMock: true}
	s := Sources{SchemaVersion: 1, INN: inn, RegisteredAt: registered, CompanyStatus: status, Credit: credit, Checks: []Source{}, SuppliedRequirements: []SuppliedRequirement{}}
	for _, kind := range []string{"REGISTRY", "CREDIT_HISTORY", "BANKRUPTCY", "ARBITRATION", "ENFORCEMENT"} {
		source := p
		source.SourceRef = "demo:" + kind + ":" + inn
		s.Checks = append(s.Checks, Source{ID: kind, Kind: kind, SubjectID: inn, Role: "BORROWER", Match: "EXACT", Status: "SUCCESS", Conclusion: "CLEAR", ValidUntil: asOf.Add(24 * time.Hour), Provenance: source})
	}
	f := &FinancialInput{Currency: "RUB", InitialCashKopecks: ptr(int64(50_000_000)), LargestCustomerShareBPS: ptr(1500), DebtScheduleKnown: ptr(true), DebtScheduleThrough: asOf.AddDate(0, 120, 0).Format("2006-01"), Months: []Month{}, DebtPayments: []DebtPayment{}, Provenance: p}
	start := periodStart(asOf)
	for i := range 12 {
		f.Months = append(f.Months, Month{Period: start.AddDate(0, i, 0).Format("2006-01"), Receipts: ptr(int64(100_000_000)), VariableCosts: ptr(int64(40_000_000)), FixedCosts: ptr(int64(15_000_000)), Taxes: ptr(int64(6_000_000)), Maintenance: ptr(int64(2_000_000)), OwnerDrawings: ptr(int64(2_000_000)), Provenance: p})
	}
	if credit.TotalDebtKopecks != nil && *credit.TotalDebtKopecks > 0 {
		balance := *credit.TotalDebtKopecks
		for i := 1; i <= 24; i++ {
			principal := balance / int64(25-i)
			interest := scale(balance, 150)
			balance -= principal
			f.DebtPayments = append(f.DebtPayments, DebtPayment{ObligationID: "demo-existing", Period: asOf.AddDate(0, i, 0).Format("2006-01"), AmountKopecks: principal + interest})
		}
	}
	s.Financials = f
	from, to := start.Format(time.DateOnly), start.AddDate(1, 0, -1).Format(time.DateOnly)
	for _, code := range []string{"CASH_FLOW", "BUSINESS_RESULT"} {
		s.SuppliedRequirements = append(s.SuppliedRequirements, SuppliedRequirement{Code: code, PeriodFrom: from, PeriodTo: to, SourceRef: p.SourceRef})
	}
	if len(inn) == 10 {
		s.SuppliedRequirements = append(s.SuppliedRequirements, SuppliedRequirement{Code: "BUSINESS_RESULT", PeriodFrom: time.Date(asOf.Year()-1, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly), PeriodTo: time.Date(asOf.Year()-1, 12, 31, 0, 0, 0, 0, time.UTC).Format(time.DateOnly), SourceRef: "demo-girbo:" + inn})
	}
	return s
}
