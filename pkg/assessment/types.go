// Package assessment is the deterministic, versioned bank assessment kernel.
// It has no database, network, or workflow side effects.
package assessment

import (
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/creditcalc"
)

const PolicyVersion = "sme-cashflow-policy-v2"
const ModelVersion = "expert-cashflow-scorecard-v2"
const SchemaVersion = 1

type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}
type Provenance struct {
	SourceRef  string    `json:"source_ref"`
	ObservedAt time.Time `json:"observed_at"`
	Quality    string    `json:"quality"`
	IsMock     bool      `json:"is_mock"`
}
type Month struct {
	Period        string     `json:"period"`
	Receipts      *int64     `json:"operating_receipts_kopecks"`
	VariableCosts *int64     `json:"variable_costs_kopecks"`
	FixedCosts    *int64     `json:"fixed_costs_kopecks"`
	Taxes         *int64     `json:"taxes_kopecks"`
	Maintenance   *int64     `json:"maintenance_capex_kopecks"`
	OwnerDrawings *int64     `json:"owner_drawings_kopecks"`
	Provenance    Provenance `json:"provenance"`
}
type DebtPayment struct {
	ObligationID  string `json:"obligation_id"`
	Period        string `json:"period"`
	AmountKopecks int64  `json:"amount_kopecks"`
}
type BalanceSheet struct {
	AsOf               string     `json:"as_of"`
	AssetsKopecks      int64      `json:"assets_kopecks"`
	LiabilitiesKopecks int64      `json:"liabilities_kopecks"`
	EquityKopecks      int64      `json:"equity_kopecks"`
	ReceivablesKopecks int64      `json:"receivables_kopecks"`
	PayablesKopecks    int64      `json:"payables_kopecks"`
	Provenance         Provenance `json:"provenance"`
}
type FinancialInput struct {
	Currency                string        `json:"currency"`
	Months                  []Month       `json:"months"`
	InitialCashKopecks      *int64        `json:"initial_cash_kopecks"`
	LargestCustomerShareBPS *int          `json:"largest_customer_share_bps"`
	DebtScheduleKnown       *bool         `json:"debt_schedule_known"`
	DebtScheduleThrough     string        `json:"debt_schedule_through"`
	DebtPayments            []DebtPayment `json:"debt_payments"`
	BalanceSheet            *BalanceSheet `json:"balance_sheet,omitempty"`
	Provenance              Provenance    `json:"provenance"`
}
type Preferences struct {
	MinimumUsefulAmountKopecks *int64 `json:"minimum_useful_amount_kopecks,omitempty"`
	MaxTermMonths              *int   `json:"max_term_months,omitempty"`
	MaxPaymentKopecks          *int64 `json:"max_payment_kopecks,omitempty"`
	Priority                   string `json:"priority"` // KEEP_AMOUNT, LOW_PAYMENT, LOW_TOTAL_COST.
}
type Submission struct {
	SchemaVersion int             `json:"schema_version"`
	Financials    *FinancialInput `json:"financials,omitempty"`
	Preferences   Preferences     `json:"preferences"`
}
type Source struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	SubjectID  string     `json:"subject_id"`
	Role       string     `json:"role"`       // BORROWER, DIRECTOR, OWNER.
	Match      string     `json:"match"`      // EXACT, AMBIGUOUS, NONE.
	Status     string     `json:"status"`     // SUCCESS, NOT_FOUND, UNAVAILABLE, STALE, AMBIGUOUS.
	Conclusion string     `json:"conclusion"` // CLEAR, REVIEW, BLOCK.
	ValidUntil time.Time  `json:"valid_until"`
	Provenance Provenance `json:"provenance"`
}
type CreditHistory struct {
	HistoricalMaxDPD *int   `json:"historical_max_dpd_12m"`
	CurrentDPD       *int   `json:"current_dpd"`
	OverdueEpisodes  *int   `json:"overdue_episodes_12m"`
	PeriodFrom       string `json:"period_from"`
	PeriodTo         string `json:"period_to"`
	TotalDebtKopecks *int64 `json:"total_debt_kopecks"`
}
type SuppliedRequirement struct {
	Code       string `json:"code"`
	PeriodFrom string `json:"period_from"`
	PeriodTo   string `json:"period_to"`
	SourceRef  string `json:"source_ref"`
}
type Sources struct {
	SchemaVersion        int                   `json:"schema_version"`
	INN                  string                `json:"inn"`
	RegisteredAt         string                `json:"registered_at"`
	CompanyStatus        string                `json:"company_status"`
	Checks               []Source              `json:"checks"`
	Credit               CreditHistory         `json:"credit"`
	Financials           *FinancialInput       `json:"financials,omitempty"`
	SuppliedRequirements []SuppliedRequirement `json:"supplied_requirements"`
}
type Product struct {
	ID        string   `json:"id"`
	Version   int64    `json:"version"`
	Name      string   `json:"name"`
	Purposes  []string `json:"purposes"`
	MinAmount int64    `json:"min_amount_kopecks"`
	MaxAmount int64    `json:"max_amount_kopecks"`
	MinTerm   int      `json:"min_term_months"`
	MaxTerm   int      `json:"max_term_months"`
	RateABPS  int      `json:"rate_a_bps"`
	RateBBPS  int      `json:"rate_b_bps"`
}
type Input struct {
	ApplicationID      string     `json:"application_id"`
	InputRevision      int64      `json:"input_revision"`
	Generation         int64      `json:"assessment_generation"`
	AsOf               time.Time  `json:"as_of"`
	INN                string     `json:"inn"`
	LegalForm          string     `json:"legal_form"`
	RequestedProductID string     `json:"requested_product_id"`
	RequestedAmount    int64      `json:"requested_amount_kopecks"`
	RequestedTerm      int        `json:"requested_term_months"`
	Purpose            string     `json:"purpose"`
	Submission         Submission `json:"submission"`
	Sources            Sources    `json:"sources"`
	Products           []Product  `json:"products"`
}
type CashMonth struct {
	Period              string   `json:"period"`
	CFADSKopecks        int64    `json:"cfads_kopecks"`
	ReceiptsKopecks     int64    `json:"operating_receipts_kopecks"`
	ExistingDebtKopecks int64    `json:"existing_debt_payment_kopecks"`
	NewDebtKopecks      int64    `json:"new_debt_payment_kopecks"`
	ClosingCashKopecks  int64    `json:"closing_cash_kopecks"`
	Quality             string   `json:"quality"`
	EvidenceRefs        []string `json:"evidence_refs"`
}
type FinancialSnapshot struct {
	ID                      string      `json:"id"`
	Currency                string      `json:"currency"`
	Unit                    string      `json:"unit"`
	PeriodFrom              string      `json:"period_from"`
	PeriodTo                string      `json:"period_to"`
	ForecastMethod          string      `json:"forecast_method"`
	History                 []CashMonth `json:"history"`
	Base                    []CashMonth `json:"base"`
	Stress                  []CashMonth `json:"stress"`
	InitialCashKopecks      int64       `json:"initial_cash_kopecks"`
	MarginBPS               int         `json:"operating_margin_bps"`
	LargestCustomerShareBPS *int        `json:"largest_customer_share_bps"`
	BalanceStatus           string      `json:"balance_status"`
	Provenance              Provenance  `json:"provenance"`
}
type Factor struct {
	ID              string   `json:"factor_id"`
	Value           int      `json:"value"`
	Unit            string   `json:"unit"`
	WeightBPS       int      `json:"weight_bps"`
	Points          int      `json:"contribution"`
	MaxPoints       int      `json:"max_points"`
	Status          string   `json:"status"`
	ExplanationCode string   `json:"explanation_code"`
	PeriodFrom      string   `json:"period_from"`
	PeriodTo        string   `json:"period_to"`
	EvidenceRefs    []string `json:"evidence_refs"`
}
type RiskResult struct {
	Status         string   `json:"status"`
	ModelVersion   string   `json:"model_version"`
	ConfigChecksum string   `json:"config_checksum"`
	Score          *int     `json:"score,omitempty"`
	Grade          string   `json:"grade,omitempty"`
	Baseline       int      `json:"baseline_score"`
	Factors        []Factor `json:"factors"`
	Issues         []Issue  `json:"issues"`
}
type CheckResult struct {
	Conclusion                      string   `json:"conclusion"`
	Codes                           []string `json:"codes"`
	Sources                         []Source `json:"sources"`
	RegistrationDateIsActivityProxy bool     `json:"registration_date_is_activity_proxy"`
}
type Candidate struct {
	ID                   string              `json:"candidate_id"`
	ProductID            string              `json:"product_id"`
	ProductVersion       int64               `json:"product_version"`
	ProductName          string              `json:"product_name"`
	Schedule             creditcalc.Schedule `json:"schedule"`
	UncoveredNeedKopecks int64               `json:"uncovered_need_kopecks"`
	Base                 []CashMonth         `json:"base"`
	Stress               []CashMonth         `json:"stress"`
	BaseCoverageBPS      int                 `json:"base_coverage_bps"`
	StressCoverageBPS    int                 `json:"stress_coverage_bps"`
	Status               string              `json:"status"`
	ValidUntil           time.Time           `json:"valid_until"`
}
type Change struct {
	PreviousAssessmentID string   `json:"previous_assessment_id"`
	CurrentAssessmentID  string   `json:"current_assessment_id"`
	PreviousPeriodFrom   string   `json:"previous_period_from,omitempty"`
	PreviousPeriodTo     string   `json:"previous_period_to,omitempty"`
	CurrentPeriodFrom    string   `json:"current_period_from,omitempty"`
	CurrentPeriodTo      string   `json:"current_period_to,omitempty"`
	Quality              string   `json:"quality"`
	EvidenceRefs         []string `json:"evidence_refs"`
	Path                 string   `json:"path"`
	Previous             any      `json:"previous"`
	Current              any      `json:"current"`
	Unit                 string   `json:"unit"`
}
type Result struct {
	SchemaVersion        int                `json:"schema_version"`
	ID                   string             `json:"assessment_id"`
	InputRevision        int64              `json:"input_revision"`
	Generation           int64              `json:"assessment_generation"`
	AsOf                 time.Time          `json:"as_of"`
	ValidUntil           time.Time          `json:"valid_until"`
	PolicyVersion        string             `json:"policy_version"`
	PolicyChecksum       string             `json:"policy_checksum"`
	InputHash            string             `json:"input_hash"`
	Status               string             `json:"status"`
	Checks               CheckResult        `json:"checks"`
	Financial            *FinancialSnapshot `json:"financial,omitempty"`
	Risk                 RiskResult         `json:"risk"`
	OriginalFailureCodes []string           `json:"original_failure_codes"`
	Candidates           []Candidate        `json:"candidates"`
	Issues               []Issue            `json:"issues"`
	Changes              []Change           `json:"changes"`
}

// PublicResult is an explicit allowlist: it contains no score, internal rules or related-person records.
type PublicResult struct {
	SchemaVersion int                `json:"schema_version"`
	ID            string             `json:"assessment_id"`
	InputRevision int64              `json:"input_revision"`
	Generation    int64              `json:"assessment_generation"`
	AsOf          time.Time          `json:"as_of"`
	ValidUntil    time.Time          `json:"valid_until"`
	PolicyVersion string             `json:"policy_version"`
	Status        string             `json:"status"`
	Financial     *FinancialSnapshot `json:"financial,omitempty"`
	Candidates    []Candidate        `json:"candidates"`
	Issues        []Issue            `json:"issues"`
	Changes       []Change           `json:"changes"`
}
