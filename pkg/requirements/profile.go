package requirements

import (
	"strconv"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
)

func day(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil && t.Format("2006-01-02") == s
}
func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func issue(path, code, message string) Issue { return Issue{path, code, message} }

func validINN(inn string) bool {
	if len(inn) != 10 && len(inn) != 12 {
		return false
	}
	d := make([]int, len(inn))
	for i, c := range inn {
		if c < '0' || c > '9' {
			return false
		}
		d[i], _ = strconv.Atoi(string(c))
	}
	check := func(weights []int) int {
		n := 0
		for i, w := range weights {
			n += d[i] * w
		}
		return n % 11 % 10
	}
	if len(d) == 10 {
		return d[9] == check([]int{2, 4, 10, 3, 5, 9, 4, 6, 8})
	}
	return d[10] == check([]int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}) && d[11] == check([]int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8})
}

func taxCombination(form commonpb.LegalForm, regimes []commonpb.TaxRegime) bool {
	if len(regimes) == 0 || len(regimes) > 2 {
		return false
	}
	seen := map[commonpb.TaxRegime]bool{}
	for _, r := range regimes {
		if r == commonpb.TaxRegime_TAX_REGIME_UNSPECIFIED || seen[r] {
			return false
		}
		seen[r] = true
		if form == commonpb.LegalForm_LEGAL_FORM_LLC && (r == commonpb.TaxRegime_TAX_REGIME_PSN || r == commonpb.TaxRegime_TAX_REGIME_NPD) {
			return false
		}
	}
	if len(regimes) == 1 {
		return true
	}
	if !seen[commonpb.TaxRegime_TAX_REGIME_PSN] || form != commonpb.LegalForm_LEGAL_FORM_IP {
		return false
	}
	return seen[commonpb.TaxRegime_TAX_REGIME_OSNO] || seen[commonpb.TaxRegime_TAX_REGIME_USN_INCOME] || seen[commonpb.TaxRegime_TAX_REGIME_USN_INCOME_EXPENSE]
}

// ValidateProfile checks contradictions, while leaving optional missing fields for Build.
func ValidateProfile(company *commonpb.CompanySnapshot, referenceDate time.Time) []Issue {
	if company == nil {
		return []Issue{issue("company", InvalidProfile, "Профиль отсутствует")}
	}
	var out []Issue
	ref := dateOnly(referenceDate)
	if ref.IsZero() {
		out = append(out, issue("reference_date", InvalidProfile, "Дата правил отсутствует"))
	}
	if !validINN(company.GetInn()) {
		out = append(out, issue("company.inn", InvalidProfile, "Некорректный ИНН"))
	}
	if company.GetLegalForm() != commonpb.LegalForm_LEGAL_FORM_UNSPECIFIED && validINN(company.GetInn()) {
		if company.GetLegalForm() == commonpb.LegalForm_LEGAL_FORM_IP && len(company.GetInn()) != 12 || company.GetLegalForm() == commonpb.LegalForm_LEGAL_FORM_LLC && len(company.GetInn()) != 10 {
			out = append(out, issue("company.legal_form", LegalFormINNMismatch, "Форма не совпадает с длиной ИНН"))
		}
	}
	if company.GetName() == "" || company.GetOkved() == "" {
		out = append(out, issue("company", InvalidProfile, "Название и ОКВЭД обязательны"))
	}
	registered, ok := day(company.GetRegisteredAt())
	if !ok || registered.After(ref) {
		out = append(out, issue("company.registered_at", InvalidProfile, "Некорректная дата регистрации"))
	}
	if company.ProfileSchemaVersion != nil && company.GetProfileSchemaVersion() != 2 {
		out = append(out, issue("company.profile_schema_version", InvalidProfile, "Неизвестная версия профиля"))
	}
	for _, m := range []struct {
		path   string
		value  *commonpb.Money
		signed bool
	}{{"company.revenue_12m", company.GetRevenue_12M(), false}, {"company.profit_12m", company.GetProfit_12M(), true}, {"company.obligations", company.GetObligations(), false}} {
		if m.value != nil && (m.value.GetCurrency() != "RUB" || !m.signed && m.value.GetAmountKopecks() < 0) {
			out = append(out, issue(m.path, InvalidProfile, "Некорректная сумма или валюта"))
		}
	}
	var previousEnd time.Time
	for i, p := range company.GetTaxPeriods() {
		path := "company.tax_periods[" + strconv.Itoa(i) + "]"
		if p == nil {
			out = append(out, issue(path, InvalidTaxPeriod, "Пустой период"))
			continue
		}
		from, fromOK := day(p.GetFrom())
		to := ref
		toOK := true
		if p.To != nil {
			to, toOK = day(p.GetTo())
		}
		if !fromOK || !toOK || to.Before(from) || from.After(ref) || to.After(ref) || ok && from.Before(registered) || !previousEnd.IsZero() && !from.After(previousEnd) || i > 0 && company.TaxPeriods[i-1] != nil && company.TaxPeriods[i-1].To == nil {
			out = append(out, issue(path, InvalidTaxPeriod, "Неверные или пересекающиеся даты"))
		}
		if !taxCombination(company.GetLegalForm(), p.GetRegimes()) {
			out = append(out, issue(path+".regimes", UnsupportedTaxCombination, "Неподдержанная комбинация режимов"))
		}
		if p.GetVatStatus() == commonpb.VatStatus_VAT_STATUS_UNSPECIFIED {
			out = append(out, issue(path+".vat_status", InvalidTaxPeriod, "Статус НДС не указан"))
		}
		previousEnd = to
	}
	if fp := company.GetFinancialPeriod(); fp != nil && (func() bool {
		f, a := day(fp.GetFrom())
		t, b := day(fp.GetTo())
		return !a || !b || t.Before(f) || t.After(ref)
	})() {
		out = append(out, issue("company.financial_period", InvalidProfile, "Некорректный финансовый период"))
	}
	if company.GetObligationsAsOf() != "" {
		d, valid := day(company.GetObligationsAsOf())
		if !valid || d.After(ref) {
			out = append(out, issue("company.obligations_as_of", InvalidProfile, "Некорректная дата обязательств"))
		}
	}
	return out
}
