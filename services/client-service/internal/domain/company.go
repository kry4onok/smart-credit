package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

const maxSafeInteger int64 = 9007199254740991

func ValidateCompany(c model.Company) error { return ValidateCompanyAt(c, time.Now().UTC()) }
func ValidateCompanyAt(c model.Company, referenceDate time.Time) error {
	if c.ProfileSchemaVersion != nil {
		if !knownProfileValues(c) {
			return &requirements.RuleError{Code: requirements.InvalidProfile, Issues: []requirements.Issue{{Path: "company", Code: requirements.InvalidProfile, Message: "Неизвестное значение профиля или неверный диапазон чисел"}}}
		}

		issues := requirements.ValidateProfile(profile.ToProto(c), referenceDate)
		if len(issues) > 0 {
			return &requirements.RuleError{Code: issues[0].Code, Issues: issues}
		}
		return nil
	}
	if c.LegalForm != nil || len(c.TaxPeriods) > 0 || c.FinancialPeriod != nil || c.FinancialBasis != nil || c.ObligationsAsOf != nil {
		return &requirements.RuleError{Code: requirements.InvalidProfile}
	}
	if c.Revenue12mKopecks == nil || c.Profit12mKopecks == nil || c.ObligationsKopecks == nil {
		return ErrInvalidCompanyProfile
	}
	if !validINN(c.INN) || c.RegisteredAt.IsZero() || utf8.RuneCountInString(strings.TrimSpace(c.Name)) < 1 || utf8.RuneCountInString(c.Name) > 256 ||
		utf8.RuneCountInString(c.OKVED) < 2 || utf8.RuneCountInString(c.OKVED) > 16 ||
		*c.Revenue12mKopecks < 0 || *c.Revenue12mKopecks > maxSafeInteger ||
		*c.Profit12mKopecks < -maxSafeInteger || *c.Profit12mKopecks > maxSafeInteger ||
		*c.ObligationsKopecks < 0 || *c.ObligationsKopecks > maxSafeInteger {
		return ErrInvalidCompanyProfile
	}
	return nil
}

func validINN(value string) bool {
	if len(value) != 10 && len(value) != 12 {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	digits := make([]int, len(value))
	for i := range value {
		digits[i] = int(value[i] - '0')
	}
	if len(digits) == 10 {
		return checksum(digits[:9], []int{2, 4, 10, 3, 5, 9, 4, 6, 8}) == digits[9]
	}
	return checksum(digits[:11], []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8, 0}) == digits[10] &&
		checksum(digits[:12], []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8, 0}) == digits[11]
}

func checksum(digits, weights []int) int {
	sum := 0
	for i, weight := range weights {
		sum += digits[i] * weight
	}
	return sum % 11 % 10
}

func knownProfileValues(c model.Company) bool {
	if utf8.RuneCountInString(strings.TrimSpace(c.Name)) < 1 || utf8.RuneCountInString(c.Name) > 256 || len(c.OKVED) < 2 || len(c.OKVED) > 16 {
		return false
	}
	for _, n := range []*int64{c.Revenue12mKopecks, c.Profit12mKopecks, c.ObligationsKopecks} {
		if n != nil && (*n > maxSafeInteger || *n < -maxSafeInteger) {
			return false
		}
	}
	if c.LegalForm != nil {
		v, ok := commonpb.LegalForm_value["LEGAL_FORM_"+*c.LegalForm]
		if !ok || v == 0 {
			return false
		}
	}
	if c.FinancialBasis != nil {
		v, ok := commonpb.FinancialBasis_value["FINANCIAL_BASIS_"+*c.FinancialBasis]
		if !ok || v == 0 {
			return false
		}
	}
	for _, p := range c.TaxPeriods {
		if v, ok := commonpb.VatStatus_value["VAT_STATUS_"+p.VATStatus]; !ok || v == 0 {
			return false
		}
		for _, r := range p.Regimes {
			if v, ok := commonpb.TaxRegime_value["TAX_REGIME_"+r]; !ok || v == 0 {
				return false
			}
		}
	}
	return true
}
