package scoring

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestBuildRequestMatchesSharedScoringFixtures(t *testing.T) {
	data, err := os.ReadFile("../../../../../seed/scoring/personas.json")
	if err != nil {
		t.Fatalf("read shared fixtures: %v", err)
	}
	var fixtures struct {
		Cases []struct {
			Name    string          `json:"name"`
			Request json.RawMessage `json:"request"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("decode shared fixtures: %v", err)
	}
	wantByName := make(map[string]*scoringpb.ScoreRequest)
	for _, fixture := range fixtures.Cases {
		if fixture.Name != "risk_a" && fixture.Name != "zero_debt" && fixture.Name != "missing_debt" {
			continue
		}
		request := &scoringpb.ScoreRequest{}
		if err := protojson.Unmarshal(fixture.Request, request); err != nil {
			t.Fatalf("decode %s request: %v", fixture.Name, err)
		}
		wantByName[fixture.Name] = request
	}

	for _, name := range []string{"risk_a", "zero_debt", "missing_debt"} {
		t.Run(name, func(t *testing.T) {
			input := validInput()
			expected := wantByName[name]
			if expected == nil {
				t.Fatalf("shared %s fixture is missing", name)
			}
			input.ApplicationID = uuid.MustParse(expected.ApplicationId)
			switch name {
			case "zero_debt":
				input.CreditHistory.TotalDebt.AmountKopecks = 0
			case "missing_debt":
				input.CreditHistory.TotalDebt = nil
			}
			actual, err := BuildRequest(input)
			if err != nil {
				t.Fatalf("BuildRequest() error = %v", err)
			}
			if !proto.Equal(actual, expected) {
				t.Fatalf("request does not match shared %s fixture\nactual: %v\nexpected: %v", name, actual, expected)
			}
		})
	}
}

func TestBuildRequestPreservesKnownZeroAndMissingDebt(t *testing.T) {
	zero := int32(0)
	input := Input{
		ApplicationID: uuid.MustParse("c1000000-0000-4000-8000-000000000001"),
		ProductID:     uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		AsOfDate:      time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
		Company: &commonpb.CompanySnapshot{
			Revenue_12M: &commonpb.Money{AmountKopecks: 0, Currency: "RUB"},
			Profit_12M:  &commonpb.Money{AmountKopecks: -100, Currency: "RUB"},
		},
		Profile: &bureaupb.CompanyProfileResponse{RegisteredAt: "2025-01-31"},
		CreditHistory: &bureaupb.CreditHistoryResponse{
			OverdueCount_12M: &zero,
			MaxOverdueDays:   &zero,
		},
	}

	request, err := BuildRequest(input)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	features := request.Features
	if features.Revenue_12M == nil || features.Revenue_12M.AmountKopecks != 0 || features.Profit_12M.AmountKopecks != -100 {
		t.Fatalf("financial features = %+v", features)
	}
	if features.TotalDebt != nil || features.CompanyAgeMonths == nil || *features.CompanyAgeMonths != 13 {
		t.Fatalf("debt and age = %+v", features)
	}
	if features.OverdueCount_12M == nil || *features.OverdueCount_12M != 0 ||
		features.MaxOverdueDays == nil || *features.MaxOverdueDays != 0 {
		t.Fatalf("explicit overdue zeros were lost: %+v", features)
	}
	if request.FeatureSchemaVersion != FeatureSchemaVersion || request.ApplicationId != input.ApplicationID.String() {
		t.Fatalf("request envelope = %+v", request)
	}
}

func TestBuildRequestRejectsInvalidSourceData(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Input)
	}{
		{name: "future registration", change: func(input *Input) {
			input.Profile.RegisteredAt = "2027-01-01"
		}},
		{name: "invalid date", change: func(input *Input) {
			input.Profile.RegisteredAt = "2026-02-30"
		}},
		{name: "wrong currency", change: func(input *Input) {
			input.Company.Revenue_12M.Currency = "USD"
		}},
		{name: "negative debt", change: func(input *Input) {
			input.CreditHistory.TotalDebt.AmountKopecks = -1
		}},
		{name: "missing enrichment", change: func(input *Input) {
			input.CreditHistory = nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput()
			tt.change(&input)
			if _, err := BuildRequest(input); err == nil {
				t.Fatal("BuildRequest() succeeded for invalid source data")
			}
		})
	}
}

func validInput() Input {
	zero := int32(0)
	return Input{
		ApplicationID: uuid.MustParse("c1000000-0000-4000-8000-000000000001"),
		ProductID:     uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		AsOfDate:      time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC),
		Company: &commonpb.CompanySnapshot{
			Revenue_12M: &commonpb.Money{AmountKopecks: 1_200_000_000, Currency: "RUB"},
			Profit_12M:  &commonpb.Money{AmountKopecks: 180_000_000, Currency: "RUB"},
		},
		Profile: &bureaupb.CompanyProfileResponse{RegisteredAt: "2023-10-22"},
		CreditHistory: &bureaupb.CreditHistoryResponse{
			TotalDebt:        &commonpb.Money{AmountKopecks: 300_000_000, Currency: "RUB"},
			OverdueCount_12M: &zero,
			MaxOverdueDays:   &zero,
		},
	}
}
