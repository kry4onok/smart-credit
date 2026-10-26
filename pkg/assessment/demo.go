package assessment

import (
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/demodata"
)

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
	if len(inn) == 10 {
		for _, role := range []string{"DIRECTOR", "OWNER"} {
			for _, kind := range []string{"BANKRUPTCY", "ENFORCEMENT"} {
				provenance := p
				provenance.SourceRef = "demo-related:" + role + ":" + kind + ":" + inn
				s.Checks = append(s.Checks, Source{ID: role + ":" + kind, Kind: kind, SubjectID: "demo-person:" + role + ":" + inn, Role: role, Match: "EXACT", Status: "NOT_FOUND", Conclusion: "CLEAR", ValidUntil: asOf.Add(24 * time.Hour), Provenance: provenance})
			}
		}
	}
	f := &FinancialInput{Currency: "RUB", InitialCashKopecks: ptr(int64(50_000_000)), LargestCustomerShareBPS: ptr(1500), DebtScheduleKnown: ptr(true), DebtScheduleThrough: asOf.AddDate(0, 120, 0).Format("2006-01"), Months: []Month{}, DebtPayments: []DebtPayment{}, Provenance: p}
	receipts, variable := int64(100_000_000), int64(40_000_000)
	if company, ok := demodata.Lookup(inn); ok {
		receipts = company.MonthlyReceiptsKopecks
		// The other cash outflows below total 250,000 RUB per month. Construct
		// actual cash components; Score still derives margin from all 12 months.
		variable = receipts - scale(receipts, company.MarginBPS) - 25_000_000
		f.InitialCashKopecks = ptr(company.InitialCashKopecks)
		f.LargestCustomerShareBPS = ptr(company.LargestCustomerShareBPS)
	}
	start := periodStart(asOf)
	for i := range 12 {
		f.Months = append(f.Months, Month{Period: start.AddDate(0, i, 0).Format("2006-01"), Receipts: ptr(receipts), VariableCosts: ptr(variable), FixedCosts: ptr(int64(15_000_000)), Taxes: ptr(int64(6_000_000)), Maintenance: ptr(int64(2_000_000)), OwnerDrawings: ptr(int64(2_000_000)), Provenance: p})
	}
	if credit.TotalDebtKopecks != nil && *credit.TotalDebtKopecks > 0 {
		balance := *credit.TotalDebtKopecks
		// Step from the first day: AddDate on the 29th–31st skips short months and
		// would put two payments into one period.
		month := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
		for i := 1; i <= 24; i++ {
			principal := balance / int64(25-i)
			interest := scale(balance, 150)
			balance -= principal
			f.DebtPayments = append(f.DebtPayments, DebtPayment{ObligationID: "demo-existing", Period: month.AddDate(0, i, 0).Format("2006-01"), AmountKopecks: principal + interest})
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
