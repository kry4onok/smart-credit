package domain

import (
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func TestValidateApplicationPatch(t *testing.T) {
	amount := int64(100)
	term := 12
	purpose := "EQUIPMENT"
	for _, tc := range []struct {
		name  string
		patch model.ApplicationPatch
		valid bool
	}{
		{name: "amount", patch: model.ApplicationPatch{AmountKopecks: &amount, AmountCurrency: "RUB"}, valid: true},
		{name: "term", patch: model.ApplicationPatch{TermMonths: &term}, valid: true},
		{name: "purpose", patch: model.ApplicationPatch{Purpose: &purpose}, valid: true},
		{name: "empty", patch: model.ApplicationPatch{}, valid: false},
		{name: "wrong currency", patch: model.ApplicationPatch{AmountKopecks: &amount, AmountCurrency: "USD"}, valid: false},
		{name: "zero amount", patch: model.ApplicationPatch{AmountKopecks: new(int64), AmountCurrency: "RUB"}, valid: false},
		{name: "invalid purpose", patch: model.ApplicationPatch{Purpose: stringPtr("OTHER")}, valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateApplicationPatch(tc.patch)
			if tc.valid && err != nil {
				t.Fatalf("err = %v", err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalidApplicationPatch) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }
