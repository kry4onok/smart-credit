// Package compat keeps legacy handlers explicit while profile-v2 persistence is implemented.
package compat

import "github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"

// LegacyProfileProblem prevents both nil dereferences and silent loss of v2 fields.
// Remove this gate only with the profile-v2 storage and requirements implementation.
func LegacyProfileProblem(profile common.CompanyProfile) (code, detail string) {
	if profile.ProfileSchemaVersion != nil || profile.LegalForm != nil || profile.TaxPeriods != nil ||
		profile.FinancialPeriod != nil || profile.FinancialBasis != nil || profile.ObligationsAsOf != nil {
		return "PROFILE_V2_NOT_ENABLED", "Контракт профиля v2 подключён; сохранение новых полей будет доступно после реализации этапа 1."
	}
	if profile.Revenue12m == nil || profile.Profit12m == nil || profile.Obligations == nil {
		return "INVALID_COMPANY_PROFILE", "Для текущего профиля укажите выручку, прибыль и остаток обязательств. Нулевое значение допускается."
	}
	return "", ""
}
