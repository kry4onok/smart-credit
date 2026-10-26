// Package demodata contains fictional companies shared by the demo seed and UI.
package demodata

import (
	_ "embed"
	"encoding/json"
)

//go:embed companies.json
var companiesJSON []byte

type Company struct {
	Number                  int    `json:"number"`
	INN                     string `json:"inn"`
	Email                   string `json:"email"`
	Name                    string `json:"name"`
	RegisteredAt            string `json:"registered_at"`
	OKVED                   string `json:"okved"`
	TaxRegime               string `json:"tax_regime"`
	MarginBPS               int    `json:"margin_bps"`
	LargestCustomerShareBPS int    `json:"largest_customer_share_bps"`
	MonthlyReceiptsKopecks  int64  `json:"monthly_receipts_kopecks"`
	InitialCashKopecks      int64  `json:"initial_cash_kopecks"`
	DebtKopecks             int64  `json:"debt_kopecks"`
	SuggestedAmountKopecks  int64  `json:"suggested_amount_kopecks"`
	SuggestedTermMonths     int    `json:"suggested_term_months"`
	Title                   string `json:"title"`
	Description             string `json:"description"`
}

// Companies returns a fresh copy so callers cannot mutate the embedded catalogue.
func Companies() []Company {
	var companies []Company
	if err := json.Unmarshal(companiesJSON, &companies); err != nil {
		panic("invalid embedded demo company catalogue: " + err.Error())
	}
	return companies
}

func Lookup(inn string) (Company, bool) {
	for _, company := range Companies() {
		if company.INN == inn {
			return company, true
		}
	}
	return Company{}, false
}
