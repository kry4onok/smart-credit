package domain

import "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"

func CanCancelApplication(status string, pendingAction *string, hasSignature bool) bool {
	if pendingAction != nil || hasSignature {
		return false
	}
	switch status {
	case "SUBMITTED", "UNDER_REVIEW", "NEEDS_INFO", "OFFER_AVAILABLE", "FORMALIZING":
		return true
	default:
		return false
	}
}

func ValidateApplicationPatch(patch model.ApplicationPatch) error {
	if patch.AmountKopecks == nil && patch.TermMonths == nil && patch.Purpose == nil && patch.Company == nil {
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
