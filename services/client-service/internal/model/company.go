package model

import "time"

type DatePeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type TaxPeriod struct {
	From      string   `json:"from"`
	To        *string  `json:"to,omitempty"`
	Regimes   []string `json:"regimes"`
	VATStatus string   `json:"vat_status"`
}
type ProfileDetails struct {
	ProfileSchemaVersion *int32      `json:"profile_schema_version,omitempty"`
	LegalForm            *string     `json:"legal_form,omitempty"`
	TaxPeriods           []TaxPeriod `json:"tax_periods,omitempty"`
	FinancialPeriod      *DatePeriod `json:"financial_period,omitempty"`
	FinancialBasis       *string     `json:"financial_basis,omitempty"`
	ObligationsAsOf      *time.Time  `json:"obligations_as_of,omitempty"`
}
type Company struct {
	ProfileDetails
	INN                string
	Name               string
	OKVED              string
	RegisteredAt       time.Time
	Revenue12mKopecks  *int64
	Profit12mKopecks   *int64
	ObligationsKopecks *int64
	Version            int64
}
