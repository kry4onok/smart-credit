package gethandler

import (
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func apiDate(value string) (openapi_types.Date, bool) {
	date, err := time.Parse("2006-01-02", value)
	return openapi_types.Date{Time: date}, err == nil && date.Format("2006-01-02") == value
}

func toDatePeriod(value *commonpb.DatePeriod) *common.DatePeriod {
	if value == nil {
		return nil
	}
	from, validFrom := apiDate(value.GetFrom())
	to, validTo := apiDate(value.GetTo())
	if !validFrom || !validTo {
		return nil
	}
	return &common.DatePeriod{From: from, To: to}
}

func toCompanyV2(value *commonpb.CompanySnapshot) common.CompanyProfile {
	registered, _ := apiDate(value.GetRegisteredAt())
	result := common.CompanyProfile{Inn: value.GetInn(), Name: value.GetName(), Okved: value.GetOkved(), RegisteredAt: registered}
	if value.ProfileSchemaVersion != nil {
		version := common.CompanyProfileProfileSchemaVersion(value.GetProfileSchemaVersion())
		result.ProfileSchemaVersion = &version
	}
	if value.GetLegalForm() != commonpb.LegalForm_LEGAL_FORM_UNSPECIFIED {
		form := common.LegalForm(strings.TrimPrefix(value.GetLegalForm().String(), "LEGAL_FORM_"))
		result.LegalForm = &form
	}
	if value.Revenue_12M != nil {
		result.Revenue12m = &common.Money{AmountKopecks: value.Revenue_12M.GetAmountKopecks(), Currency: common.MoneyCurrencyRUB}
	}
	if value.Profit_12M != nil {
		result.Profit12m = &common.SignedMoney{AmountKopecks: value.Profit_12M.GetAmountKopecks(), Currency: common.SignedMoneyCurrencyRUB}
	}
	if value.Obligations != nil {
		result.Obligations = &common.Money{AmountKopecks: value.Obligations.GetAmountKopecks(), Currency: common.MoneyCurrencyRUB}
	}
	if value.TaxPeriods != nil {
		periods := make([]common.TaxPeriod, 0, len(value.TaxPeriods))
		for _, p := range value.TaxPeriods {
			if p == nil {
				continue
			}
			from, ok := apiDate(p.GetFrom())
			if !ok {
				continue
			}
			item := common.TaxPeriod{From: from, Regimes: []common.TaxRegime{}, VatStatus: common.TaxPeriodVatStatus(strings.TrimPrefix(p.GetVatStatus().String(), "VAT_STATUS_"))}
			if p.To != nil {
				to, valid := apiDate(p.GetTo())
				if valid {
					item.To = &to
				}
			}
			for _, regime := range p.GetRegimes() {
				item.Regimes = append(item.Regimes, common.TaxRegime(strings.TrimPrefix(regime.String(), "TAX_REGIME_")))
			}
			periods = append(periods, item)
		}
		result.TaxPeriods = &periods
	}
	result.FinancialPeriod = toDatePeriod(value.GetFinancialPeriod())
	if value.GetFinancialBasis() != commonpb.FinancialBasis_FINANCIAL_BASIS_UNSPECIFIED {
		basis := common.CompanyProfileFinancialBasis(strings.TrimPrefix(value.GetFinancialBasis().String(), "FINANCIAL_BASIS_"))
		result.FinancialBasis = &basis
	}
	if value.GetObligationsAsOf() != "" {
		date, ok := apiDate(value.GetObligationsAsOf())
		if ok {
			result.ObligationsAsOf = &date
		}
	}
	return result
}

func toRequirements(items []*commonpb.Requirement) []common.Requirement {
	result := make([]common.Requirement, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		r := common.Requirement{Id: item.GetId(), Code: item.GetCode(), Label: item.GetLabel(), Reason: item.GetReason(), Kind: common.RequirementKind(strings.TrimPrefix(item.GetKind().String(), "REQUIREMENT_KIND_")), RequiredAt: common.RequirementRequiredAt(strings.TrimPrefix(item.GetRequiredAt().String(), "REQUIREMENT_STAGE_")), Status: common.RequirementStatus(strings.TrimPrefix(item.GetStatus().String(), "REQUIREMENT_STATUS_")), AcceptedDocumentTypes: []common.DocumentType{}, DocumentIds: []common.Id{}}
		if item.GetFieldPath() != "" {
			path := item.GetFieldPath()
			r.FieldPath = &path
		}
		r.Period = toDatePeriod(item.GetPeriod())
		if item.GetTaxRegime() != commonpb.TaxRegime_TAX_REGIME_UNSPECIFIED {
			regime := common.TaxRegime(strings.TrimPrefix(item.GetTaxRegime().String(), "TAX_REGIME_"))
			r.TaxRegime = &regime
		}
		for _, typ := range item.GetAcceptedDocumentTypes() {
			r.AcceptedDocumentTypes = append(r.AcceptedDocumentTypes, common.DocumentType(strings.TrimPrefix(typ.String(), "DOCUMENT_TYPE_")))
		}
		for _, id := range item.GetDocumentIds() {
			parsed, err := uuid.Parse(id)
			if err == nil {
				r.DocumentIds = append(r.DocumentIds, parsed)
			}
		}
		result = append(result, r)
	}
	return result
}

func toRequirementsPlan(plan *commonpb.RequirementsPlan) *common.RequirementsPlan {
	if plan == nil {
		return nil
	}
	referenceDate, ok := apiDate(plan.GetReferenceDate())
	if !ok {
		return nil
	}
	return &common.RequirementsPlan{PolicyVersion: common.RequirementsPlanPolicyVersion(plan.GetPolicyVersion()), ReferenceDate: referenceDate, CanSubmit: false, Route: common.RequirementsPlanRoute(strings.TrimPrefix(plan.GetRoute().String(), "REQUIREMENTS_ROUTE_")), ReasonCodes: append([]string{}, plan.GetReasonCodes()...), Items: toRequirements(plan.GetItems())}
}
