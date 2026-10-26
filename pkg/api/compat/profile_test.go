package compat

import (
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"testing"
)

func TestLegacyProfilePresence(t *testing.T) {
	p := common.CompanyProfile{Revenue12m: &common.Money{}, Profit12m: &common.SignedMoney{}, Obligations: &common.Money{}}
	if code, _ := LegacyProfileProblem(p); code != "" {
		t.Fatal("explicit zero rejected", code)
	}
	p.Profit12m = nil
	if code, _ := LegacyProfileProblem(p); code != "INVALID_COMPANY_PROFILE" {
		t.Fatal("missing accepted", code)
	}
	version := common.N2
	p.ProfileSchemaVersion = &version
	if code, _ := LegacyProfileProblem(p); code != "PROFILE_V2_NOT_ENABLED" {
		t.Fatal("v2 fields would be discarded", code)
	}
}
