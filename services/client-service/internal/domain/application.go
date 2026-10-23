package domain

import "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"

func ValidateApplicationPatch(patch model.ApplicationPatch) error {
	if patch.AmountKopecks == nil && patch.TermMonths == nil && patch.Purpose == nil {
		return ErrInvalidApplicationPatch
	}
	if patch.AmountKopecks != nil && (patch.AmountCurrency != "RUB" || *patch.AmountKopecks < 1 || *patch.AmountKopecks > maxSafeInteger) {
		return ErrInvalidApplicationPatch
	}
	if patch.TermMonths != nil && *patch.TermMonths < 1 {
		return ErrInvalidApplicationPatch
	}
	if patch.Purpose != nil && !validPurpose(*patch.Purpose) {
		return ErrInvalidApplicationPatch
	}
	return nil
}

func validPurpose(value string) bool {
	switch value {
	case "WORKING_CAPITAL", "EQUIPMENT", "EXPANSION":
		return true
	default:
		return false
	}
}
