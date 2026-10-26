package requirements

import (
	"sort"
	"strings"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
)

const policyVersion = "sme-profile-v1"

var fieldLabels = map[string]string{
	"company.legal_form":        "Организационная форма",
	"company.tax_periods":       "Налоговый режим на дату заявки",
	"company.revenue_12m":       "Выручка за финансовый период",
	"company.profit_12m":        "Прибыль за финансовый период",
	"company.obligations":       "Остаток кредитных обязательств",
	"company.financial_period":  "Финансовый период",
	"company.financial_basis":   "Основа финансовых данных",
	"company.obligations_as_of": "Дата остатка обязательств",
	"form.amount":               "Сумма кредита",
	"form.term_months":          "Срок кредита",
	"form.purpose":              "Цель кредита",
}

var documentLabels = map[string]string{
	"CASH_FLOW":            "Банковская выписка",
	"BUSINESS_RESULT":      "Подтверждение результатов бизнеса",
	"PATENT_RIGHT":         "Действующий патент",
	"RECEIVABLES_PAYABLES": "Расшифровка дебиторской и кредиторской задолженности",
	"MANAGEMENT_REPORT":    "Управленческий отчёт",
	"TAX_DECLARATION":      "Налоговая декларация",
	"INCOME_LEDGER":        "Книга учёта доходов и расходов",
}

func field(path string, present bool) *commonpb.Requirement {
	code := strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
	status := commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING
	if present {
		status = commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED
	}
	return &commonpb.Requirement{Id: strings.ToLower(code), Code: code, Label: fieldLabels[path], Reason: "Необходимо для рассмотрения заявки", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_FIELD, RequiredAt: commonpb.RequirementStage_REQUIREMENT_STAGE_SUBMIT, Status: status, FieldPath: path}
}

func doc(code string, stage commonpb.RequirementStage, period *commonpb.DatePeriod, regime commonpb.TaxRegime, types ...commonpb.DocumentType) *commonpb.Requirement {
	regimeID := "all"
	if regime != commonpb.TaxRegime_TAX_REGIME_UNSPECIFIED {
		regimeID = strings.ToLower(strings.TrimPrefix(regime.String(), "TAX_REGIME_"))
	}
	id := strings.ToLower(code) + ":" + period.GetFrom() + ":" + period.GetTo() + ":" + regimeID
	return &commonpb.Requirement{Id: id, Code: code, Label: documentLabels[code], Reason: "Файл требуется для проверки заявленных данных за указанный период", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT, RequiredAt: stage, Status: commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING, AcceptedDocumentTypes: types, Period: period, TaxRegime: regime}
}

func taxAt(periods []*commonpb.TaxPeriod, at time.Time) *commonpb.TaxPeriod {
	for _, p := range periods {
		if p == nil {
			continue
		}
		from, ok := day(p.GetFrom())
		if !ok || from.After(at) {
			continue
		}
		if p.To == nil {
			return p
		}
		to, ok := day(p.GetTo())
		if ok && !to.Before(at) {
			return p
		}
	}
	return nil
}
func has(regimes []commonpb.TaxRegime, target commonpb.TaxRegime) bool {
	for _, r := range regimes {
		if r == target {
			return true
		}
	}
	return false
}
func supported(regimes []commonpb.TaxRegime) bool {
	for _, r := range regimes {
		switch r {
		case commonpb.TaxRegime_TAX_REGIME_OSNO, commonpb.TaxRegime_TAX_REGIME_USN_INCOME, commonpb.TaxRegime_TAX_REGIME_USN_INCOME_EXPENSE, commonpb.TaxRegime_TAX_REGIME_PSN:
		default:
			return false
		}
	}
	return true
}
func period(from, to time.Time) *commonpb.DatePeriod {
	return &commonpb.DatePeriod{From: from.Format("2006-01-02"), To: to.Format("2006-01-02")}
}

func historyComplete(company *commonpb.CompanySnapshot, from, to time.Time) bool {
	registered, ok := day(company.GetRegisteredAt())
	if !ok {
		return false
	}
	if from.Before(registered) {
		from = registered
	}
	for cursor := from; !cursor.After(to); {
		p := taxAt(company.GetTaxPeriods(), cursor)
		if p == nil {
			return false
		}
		if p.To == nil {
			return true
		}
		end, ok := day(p.GetTo())
		if !ok {
			return false
		}
		cursor = end.AddDate(0, 0, 1)
	}
	return true
}

// Build produces a deterministic plan. Missing submit fields are requirements; contradictory inputs are errors.
func Build(in Input) (*commonpb.RequirementsPlan, error) {
	if in.Product == nil || in.Product.GetRequirementsPolicyVersion() != policyVersion {
		return nil, &RuleError{Code: InvalidProfile, Issues: []Issue{issue("product.requirements_policy_version", InvalidProfile, "Неизвестная версия правил")}}
	}
	ref := dateOnly(in.ReferenceDate)
	plan := &commonpb.RequirementsPlan{PolicyVersion: policyVersion, ReferenceDate: ref.Format("2006-01-02"), Route: commonpb.RequirementsRoute_REQUIREMENTS_ROUTE_STANDARD, Items: []*commonpb.Requirement{}, ReasonCodes: []string{}}
	c := in.Company
	if c == nil {
		c = &commonpb.CompanySnapshot{}
	}
	f := in.Form
	if f == nil {
		f = &commonpb.ApplicationForm{}
	}
	active := taxAt(c.GetTaxPeriods(), ref)
	taxPresent := active != nil && len(active.GetRegimes()) > 0
	legalPresent := c.GetLegalForm() != commonpb.LegalForm_LEGAL_FORM_UNSPECIFIED
	finance := c.GetFinancialPeriod()
	fields := []*commonpb.Requirement{
		field("company.legal_form", legalPresent), field("company.tax_periods", taxPresent), field("company.revenue_12m", c.Revenue_12M != nil), field("company.profit_12m", c.Profit_12M != nil), field("company.obligations", c.Obligations != nil), field("company.financial_period", finance != nil), field("company.financial_basis", c.GetFinancialBasis() != commonpb.FinancialBasis_FINANCIAL_BASIS_UNSPECIFIED), field("company.obligations_as_of", c.GetObligationsAsOf() != ""), field("form.amount", f.GetAmount() != nil), field("form.term_months", f.GetTermMonths() > 0), field("form.purpose", f.GetPurpose() != commonpb.PurposeTag_PURPOSE_TAG_UNSPECIFIED),
	}
	plan.Items = append(plan.Items, fields...)
	plan.CanSubmit = true
	for _, r := range fields {
		if r.GetStatus() != commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED {
			plan.CanSubmit = false
		}
	}
	if amount := f.GetAmount(); amount != nil {
		if amount.GetCurrency() != "RUB" || in.Product.GetMinAmount() == nil || in.Product.GetMaxAmount() == nil || amount.GetAmountKopecks() < in.Product.GetMinAmount().GetAmountKopecks() || amount.GetAmountKopecks() > in.Product.GetMaxAmount().GetAmountKopecks() {
			plan.CanSubmit = false
		}
	}
	if f.GetTermMonths() > 0 && (f.GetTermMonths() < in.Product.GetMinTermMonths() || f.GetTermMonths() > in.Product.GetMaxTermMonths()) {
		plan.CanSubmit = false
	}
	if f.GetPurpose() != commonpb.PurposeTag_PURPOSE_TAG_UNSPECIFIED {
		allowed := false
		for _, p := range in.Product.GetPurposeTags() {
			if p == f.GetPurpose() {
				allowed = true
			}
		}
		if !allowed {
			plan.CanSubmit = false
		}
	}
	var validation []Issue
	if in.Company != nil {
		validation = ValidateProfile(in.Company, ref)
	} else {
		validation = []Issue{issue("company", InvalidProfile, "Профиль отсутствует")}
	}
	if len(validation) > 0 {
		plan.CanSubmit = false
	}
	manual := func(reason string) {
		plan.Route = commonpb.RequirementsRoute_REQUIREMENTS_ROUTE_MANUAL_REVIEW
		for _, v := range plan.ReasonCodes {
			if v == reason {
				return
			}
		}
		plan.ReasonCodes = append(plan.ReasonCodes, reason)
	}
	var from, to time.Time
	validFinance := false
	if finance != nil {
		var a, b bool
		from, a = day(finance.GetFrom())
		to, b = day(finance.GetTo())
		validFinance = a && b && !to.Before(from)
	}
	if validFinance {
		standardTo := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
		standardFrom := standardTo.AddDate(0, -12, 1)
		if !from.Equal(standardFrom) || !to.Equal(standardTo) {
			manual("FINANCIAL_PERIOD_UNSUPPORTED")
		}
		if !historyComplete(c, from, ref) {
			manual("TAX_HISTORY_INCOMPLETE")
		}
	}
	if c.GetFinancialBasis() != commonpb.FinancialBasis_FINANCIAL_BASIS_UNSPECIFIED && c.GetFinancialBasis() != commonpb.FinancialBasis_FINANCIAL_BASIS_MANAGEMENT {
		manual("FINANCIAL_BASIS_UNSUPPORTED")
	}
	if d, ok := day(c.GetObligationsAsOf()); ok && ref.Sub(d) > 31*24*time.Hour {
		manual("DEBT_DATE_STALE")
	}
	if active != nil && !supported(active.GetRegimes()) {
		manual("TAX_REGIME_REQUIRES_REVIEW")
	}
	if plan.Route == commonpb.RequirementsRoute_REQUIREMENTS_ROUTE_STANDARD && validFinance && active != nil && legalPresent {
		r := period(from, to)
		y := ref.Year() - 1
		year := period(time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC))
		point := period(ref, ref)
		plan.Items = append(plan.Items, doc("CASH_FLOW", commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT, r, 0, commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT))
		registered, ok := day(c.GetRegisteredAt())
		annual := ok && !registered.After(time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)) && (ref.Month() > time.June || ref.Month() == time.June)
		if c.GetLegalForm() == commonpb.LegalForm_LEGAL_FORM_LLC && annual {
			plan.Items = append(plan.Items, doc("BUSINESS_RESULT", commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT, year, 0, commonpb.DocumentType_DOCUMENT_TYPE_ACCOUNTING_REPORT))
		} else {
			plan.Items = append(plan.Items, doc("BUSINESS_RESULT", commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT, r, 0, commonpb.DocumentType_DOCUMENT_TYPE_MANAGEMENT_REPORT))
		}
		if c.GetLegalForm() == commonpb.LegalForm_LEGAL_FORM_IP && has(active.GetRegimes(), commonpb.TaxRegime_TAX_REGIME_PSN) {
			plan.Items = append(plan.Items, doc("PATENT_RIGHT", commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT, point, commonpb.TaxRegime_TAX_REGIME_PSN, commonpb.DocumentType_DOCUMENT_TYPE_PATENT))
		}
		if c.GetLegalForm() == commonpb.LegalForm_LEGAL_FORM_LLC {
			plan.Items = append(plan.Items, doc("RECEIVABLES_PAYABLES", commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST, point, 0, commonpb.DocumentType_DOCUMENT_TYPE_RECEIVABLES_PAYABLES))
			if annual {
				plan.Items = append(plan.Items, doc("MANAGEMENT_REPORT", commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST, r, 0, commonpb.DocumentType_DOCUMENT_TYPE_MANAGEMENT_REPORT))
			}
		} else {
			plan.Items = append(plan.Items, doc("MANAGEMENT_REPORT", commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST, r, 0, commonpb.DocumentType_DOCUMENT_TYPE_MANAGEMENT_REPORT))
		}
		if annual {
			for _, reg := range []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_OSNO, commonpb.TaxRegime_TAX_REGIME_USN_INCOME, commonpb.TaxRegime_TAX_REGIME_USN_INCOME_EXPENSE} {
				if !regimeOccurred(c.GetTaxPeriods(), reg, year) {
					continue
				}
				if reg == commonpb.TaxRegime_TAX_REGIME_OSNO && c.GetLegalForm() == commonpb.LegalForm_LEGAL_FORM_IP {
					plan.Items = append(plan.Items, doc("TAX_DECLARATION", commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST, year, reg, commonpb.DocumentType_DOCUMENT_TYPE_TAX_DECLARATION_3NDFL))
				} else if reg != commonpb.TaxRegime_TAX_REGIME_OSNO {
					plan.Items = append(plan.Items, doc("TAX_DECLARATION", commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST, year, reg, commonpb.DocumentType_DOCUMENT_TYPE_TAX_DECLARATION_USN))
				}
			}
		}
		if c.GetLegalForm() == commonpb.LegalForm_LEGAL_FORM_IP {
			for _, p := range c.GetTaxPeriods() {
				if p == nil {
					continue
				}
				pf, ok := day(p.GetFrom())
				if !ok {
					continue
				}
				pt := ref
				if p.To != nil {
					pt, _ = day(p.GetTo())
				}
				if pf.Before(from) {
					pf = from
				}
				if pt.After(to) {
					pt = to
				}
				if pt.Before(pf) {
					continue
				}
				for _, reg := range p.GetRegimes() {
					if reg == commonpb.TaxRegime_TAX_REGIME_PSN || reg == commonpb.TaxRegime_TAX_REGIME_OSNO || reg == commonpb.TaxRegime_TAX_REGIME_USN_INCOME || reg == commonpb.TaxRegime_TAX_REGIME_USN_INCOME_EXPENSE {
						plan.Items = append(plan.Items, doc("INCOME_LEDGER", commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST, period(pf, pt), reg, commonpb.DocumentType_DOCUMENT_TYPE_INCOME_LEDGER))
					}
				}
			}
		}
	}
	for _, r := range plan.Items {
		if r.GetKind() == commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT {
			r.DocumentIds = MatchDocuments(r, in.Documents)
			if len(r.DocumentIds) > 0 {
				r.Status = commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED
			}
		}
	}
	sort.Slice(plan.Items, func(i, j int) bool {
		a, b := plan.Items[i], plan.Items[j]
		if a.GetRequiredAt() != b.GetRequiredAt() {
			return a.GetRequiredAt() < b.GetRequiredAt()
		}
		return a.GetId() < b.GetId()
	})
	if len(validation) > 0 {
		return plan, &RuleError{Code: validation[0].Code, Issues: validation}
	}
	return plan, nil
}

func regimeOccurred(periods []*commonpb.TaxPeriod, regime commonpb.TaxRegime, target *commonpb.DatePeriod) bool {
	start, _ := day(target.GetFrom())
	end, _ := day(target.GetTo())
	for _, p := range periods {
		if p == nil || !has(p.GetRegimes(), regime) {
			continue
		}
		from, ok := day(p.GetFrom())
		if !ok {
			continue
		}
		to := end
		if p.To != nil {
			to, _ = day(p.GetTo())
		}
		if !from.After(end) && !to.Before(start) {
			return true
		}
	}
	return false
}
