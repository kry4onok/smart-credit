package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func TestValidateCompany(t *testing.T) {
	base := model.Company{
		INN:                "7707083893",
		Name:               "Демо",
		OKVED:              "62.01",
		RegisteredAt:       time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		Revenue12mKopecks:  100,
		Profit12mKopecks:   -10,
		ObligationsKopecks: 20,
	}
	if err := ValidateCompany(base); err != nil {
		t.Fatalf("valid 10-digit INN rejected: %v", err)
	}
	base.INN = "500100732259"
	if err := ValidateCompany(base); err != nil {
		t.Fatalf("valid 12-digit INN rejected: %v", err)
	}
	for _, mutate := range []func(*model.Company){
		func(c *model.Company) { c.INN = "7707083894" },
		func(c *model.Company) { c.Name = "  " },
		func(c *model.Company) { c.RegisteredAt = time.Time{} },
		func(c *model.Company) { c.Revenue12mKopecks = -1 },
		func(c *model.Company) { c.Profit12mKopecks = maxSafeInteger + 1 },
	} {
		invalid := base
		mutate(&invalid)
		if err := ValidateCompany(invalid); !errors.Is(err, ErrInvalidCompanyProfile) {
			t.Fatalf("invalid profile error = %v", err)
		}
	}
}
