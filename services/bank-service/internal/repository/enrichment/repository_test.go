package enrichmentrepository

import (
	"encoding/json"
	"testing"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
)

func TestPreparedCreditPreservesPresenceForScoring(t *testing.T) {
	zero := int32(0)
	for _, tc := range []struct {
		name        string
		credit      *bureaupb.CreditHistoryResponse
		wantDebt    bool
		wantOverdue bool
	}{
		{"known zero", &bureaupb.CreditHistoryResponse{
			TotalDebt: &commonpb.Money{Currency: "RUB"}, OverdueCount_12M: &zero, MaxOverdueDays: &zero,
		}, true, true},
		{"missing", &bureaupb.CreditHistoryResponse{}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := prepareSource(tc.credit, "")
			if err != nil || result.outcome != "SUCCESS" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(result.resultJSON.(string)), &decoded); err != nil {
				t.Fatal(err)
			}
			_, hasDebt := decoded["total_debt"]
			_, hasOverdue := decoded["overdue_count_12m"]
			if hasDebt != tc.wantDebt || hasOverdue != tc.wantOverdue {
				t.Fatalf("presence changed in %v", decoded)
			}
		})
	}
}

func TestPreparedSourceRequiresOneOutcome(t *testing.T) {
	if _, err := prepareSource(nil, ""); err == nil {
		t.Fatal("missing response and error accepted")
	}
	if _, err := prepareSource(&bureaupb.CompanyProfileResponse{}, "BUREAU_TIMEOUT"); err == nil {
		t.Fatal("response and error accepted together")
	}
	result, err := prepareSource(nil, "BUREAU_TIMEOUT")
	if err != nil || result.outcome != "ERROR" || result.resultJSON != nil {
		t.Fatalf("error result=%+v err=%v", result, err)
	}
}

func TestPreparedProfileKeepsFieldsForCardAndScoring(t *testing.T) {
	result, err := prepareSource(&bureaupb.CompanyProfileResponse{
		Status: bureaupb.CompanyStatus_COMPANY_STATUS_ACTIVE, RegisteredAt: "2019-04-15", Okved: "10.71",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(result.resultJSON.(string)), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["status"] != "COMPANY_STATUS_ACTIVE" || decoded["registered_at"] != "2019-04-15" || decoded["okved"] != "10.71" {
		t.Fatalf("stored profile = %v", decoded)
	}
}
