// Package profile converts client profiles without replacing absent values with zero.
package profile

import (
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func Ptr[T any](v T) *T { return &v }
func FromHTTP(v common.CompanyProfile) model.Company {
	c := model.Company{INN: v.Inn, Name: v.Name, OKVED: v.Okved, RegisteredAt: v.RegisteredAt.Time}
	if v.Revenue12m != nil {
		c.Revenue12mKopecks = Ptr(v.Revenue12m.AmountKopecks)
	}
	if v.Profit12m != nil {
		c.Profit12mKopecks = Ptr(v.Profit12m.AmountKopecks)
	}
	if v.Obligations != nil {
		c.ObligationsKopecks = Ptr(v.Obligations.AmountKopecks)
	}
	if v.ProfileSchemaVersion != nil {
		c.ProfileSchemaVersion = Ptr(int32(*v.ProfileSchemaVersion))
	}
	if v.LegalForm != nil {
		c.LegalForm = Ptr(string(*v.LegalForm))
	}
	if v.FinancialBasis != nil {
		c.FinancialBasis = Ptr(string(*v.FinancialBasis))
	}
	if v.ObligationsAsOf != nil {
		c.ObligationsAsOf = Ptr(v.ObligationsAsOf.Time)
	}
	if v.FinancialPeriod != nil {
		c.FinancialPeriod = &model.DatePeriod{From: v.FinancialPeriod.From.String(), To: v.FinancialPeriod.To.String()}
	}
	if v.TaxPeriods != nil {
		for _, p := range *v.TaxPeriods {
			t := model.TaxPeriod{From: p.From.String(), VATStatus: string(p.VatStatus), Regimes: []string{}}
			if p.To != nil {
				t.To = Ptr(p.To.String())
			}
			for _, r := range p.Regimes {
				t.Regimes = append(t.Regimes, string(r))
			}
			c.TaxPeriods = append(c.TaxPeriods, t)
		}
	}
	return c
}
func ToHTTP(c model.Company) common.CompanyProfile {
	v := common.CompanyProfile{Inn: c.INN, Name: c.Name, Okved: c.OKVED, RegisteredAt: openapi_types.Date{Time: c.RegisteredAt}}
	if c.Revenue12mKopecks != nil {
		v.Revenue12m = &common.Money{AmountKopecks: *c.Revenue12mKopecks, Currency: common.MoneyCurrencyRUB}
	}
	if c.Profit12mKopecks != nil {
		v.Profit12m = &common.SignedMoney{AmountKopecks: *c.Profit12mKopecks, Currency: common.SignedMoneyCurrencyRUB}
	}
	if c.ObligationsKopecks != nil {
		v.Obligations = &common.Money{AmountKopecks: *c.ObligationsKopecks, Currency: common.MoneyCurrencyRUB}
	}
	if c.ProfileSchemaVersion != nil {
		v.ProfileSchemaVersion = Ptr(common.CompanyProfileProfileSchemaVersion(*c.ProfileSchemaVersion))
	}
	if c.LegalForm != nil {
		v.LegalForm = Ptr(common.LegalForm(*c.LegalForm))
	}
	if c.FinancialBasis != nil {
		v.FinancialBasis = Ptr(common.CompanyProfileFinancialBasis(*c.FinancialBasis))
	}
	if c.ObligationsAsOf != nil {
		v.ObligationsAsOf = &openapi_types.Date{Time: *c.ObligationsAsOf}
	}
	if c.FinancialPeriod != nil {
		v.FinancialPeriod = &common.DatePeriod{From: date(c.FinancialPeriod.From), To: date(c.FinancialPeriod.To)}
	}
	if c.TaxPeriods != nil {
		periods := make([]common.TaxPeriod, 0, len(c.TaxPeriods))
		for _, p := range c.TaxPeriods {
			t := common.TaxPeriod{From: date(p.From), VatStatus: common.TaxPeriodVatStatus(p.VATStatus), Regimes: []common.TaxRegime{}}
			if p.To != nil {
				t.To = Ptr(date(*p.To))
			}
			for _, r := range p.Regimes {
				t.Regimes = append(t.Regimes, common.TaxRegime(r))
			}
			periods = append(periods, t)
		}
		v.TaxPeriods = &periods
	}
	return v
}
func date(s string) openapi_types.Date {
	t, _ := time.Parse(time.DateOnly, s)
	return openapi_types.Date{Time: t}
}
func Money(v *int64) *commonpb.Money {
	if v == nil {
		return nil
	}
	return &commonpb.Money{AmountKopecks: *v, Currency: "RUB"}
}
func ToProto(c model.Company) *commonpb.CompanySnapshot {
	v := &commonpb.CompanySnapshot{Inn: c.INN, Name: c.Name, Okved: c.OKVED, RegisteredAt: c.RegisteredAt.Format(time.DateOnly), Revenue_12M: Money(c.Revenue12mKopecks), Profit_12M: Money(c.Profit12mKopecks), Obligations: Money(c.ObligationsKopecks), ProfileSchemaVersion: c.ProfileSchemaVersion}
	if c.LegalForm != nil {
		v.LegalForm = commonpb.LegalForm(commonpb.LegalForm_value["LEGAL_FORM_"+*c.LegalForm])
	}
	if c.FinancialBasis != nil {
		v.FinancialBasis = commonpb.FinancialBasis(commonpb.FinancialBasis_value["FINANCIAL_BASIS_"+*c.FinancialBasis])
	}
	if c.ObligationsAsOf != nil {
		v.ObligationsAsOf = c.ObligationsAsOf.Format(time.DateOnly)
	}
	if c.FinancialPeriod != nil {
		v.FinancialPeriod = &commonpb.DatePeriod{From: c.FinancialPeriod.From, To: c.FinancialPeriod.To}
	}
	for _, p := range c.TaxPeriods {
		t := &commonpb.TaxPeriod{From: p.From, To: p.To, VatStatus: commonpb.VatStatus(commonpb.VatStatus_value["VAT_STATUS_"+p.VATStatus])}
		for _, r := range p.Regimes {
			t.Regimes = append(t.Regimes, commonpb.TaxRegime(commonpb.TaxRegime_value["TAX_REGIME_"+r]))
		}
		v.TaxPeriods = append(v.TaxPeriods, t)
	}
	return v
}
func FromProto(v *commonpb.CompanySnapshot) model.Company {
	c := model.Company{INN: v.GetInn(), Name: v.GetName(), OKVED: v.GetOkved(), RegisteredAt: date(v.GetRegisteredAt()).Time}
	c.ProfileSchemaVersion = v.ProfileSchemaVersion
	if v.Revenue_12M != nil {
		c.Revenue12mKopecks = Ptr(v.Revenue_12M.AmountKopecks)
	}
	if v.Profit_12M != nil {
		c.Profit12mKopecks = Ptr(v.Profit_12M.AmountKopecks)
	}
	if v.Obligations != nil {
		c.ObligationsKopecks = Ptr(v.Obligations.AmountKopecks)
	}
	if v.LegalForm != 0 {
		c.LegalForm = Ptr(strings.TrimPrefix(v.LegalForm.String(), "LEGAL_FORM_"))
	}
	if v.FinancialBasis != 0 {
		c.FinancialBasis = Ptr(strings.TrimPrefix(v.FinancialBasis.String(), "FINANCIAL_BASIS_"))
	}
	if v.ObligationsAsOf != "" {
		c.ObligationsAsOf = Ptr(date(v.ObligationsAsOf).Time)
	}
	if v.FinancialPeriod != nil {
		c.FinancialPeriod = &model.DatePeriod{From: v.FinancialPeriod.From, To: v.FinancialPeriod.To}
	}
	for _, p := range v.TaxPeriods {
		t := model.TaxPeriod{From: p.From, To: p.To, VATStatus: strings.TrimPrefix(p.VatStatus.String(), "VAT_STATUS_")}
		for _, r := range p.Regimes {
			t.Regimes = append(t.Regimes, strings.TrimPrefix(r.String(), "TAX_REGIME_"))
		}
		c.TaxPeriods = append(c.TaxPeriods, t)
	}
	return c
}

func ValidCurrencies(v common.CompanyProfile) bool {
	return (v.Revenue12m == nil || v.Revenue12m.Currency == common.MoneyCurrencyRUB) && (v.Profit12m == nil || v.Profit12m.Currency == common.SignedMoneyCurrencyRUB) && (v.Obligations == nil || v.Obligations.Currency == common.MoneyCurrencyRUB)
}
