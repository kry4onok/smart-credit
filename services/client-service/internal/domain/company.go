package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

const maxSafeInteger int64 = 9007199254740991

func ValidateCompany(c model.Company) error {
	if !validINN(c.INN) || c.RegisteredAt.IsZero() || utf8.RuneCountInString(strings.TrimSpace(c.Name)) < 1 || utf8.RuneCountInString(c.Name) > 256 ||
		utf8.RuneCountInString(c.OKVED) < 2 || utf8.RuneCountInString(c.OKVED) > 16 ||
		c.Revenue12mKopecks < 0 || c.Revenue12mKopecks > maxSafeInteger ||
		c.Profit12mKopecks < -maxSafeInteger || c.Profit12mKopecks > maxSafeInteger ||
		c.ObligationsKopecks < 0 || c.ObligationsKopecks > maxSafeInteger {
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
