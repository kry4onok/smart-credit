package scoringrepository

import (
	"testing"
	"time"

	bureaupb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/bureau"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	scoringclient "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/client/scoring"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestSavedCreditHistoryPreservesMissingAndKnownZero(t *testing.T) {
	company := []byte(`{"revenue_12m":{"amount_kopecks":840000000,"currency":"RUB"},"profit_12m":{"amount_kopecks":96000000,"currency":"RUB"}}`)
	profile := []byte(`{"status":"ACTIVE","registered_at":"2019-04-15"}`)
	caseID := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	productID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	for _, tc := range []struct {
		name        string
		credit      string
		wantDebt    bool
		wantOverdue bool
	}{
		{"known zero", `{"total_debt":{"amount_kopecks":0,"currency":"RUB"},"overdue_count_12m":0}`, true, true},
		{"missing", `{}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, err := decodeSavedInput(caseID, productID, company, profile, []byte(tc.credit))
			if err != nil {
				t.Fatal(err)
			}
			input.AsOfDate = time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
			request, err := scoringclient.BuildRequest(input)
			if err != nil {
				t.Fatal(err)
			}
			if (request.Features.TotalDebt != nil) != tc.wantDebt ||
				(request.Features.OverdueCount_12M != nil) != tc.wantOverdue {
				t.Fatalf("presence lost: %+v", request.Features)
			}
			if tc.wantDebt && request.Features.TotalDebt.AmountKopecks != 0 {
				t.Fatalf("known zero debt changed: %+v", request.Features.TotalDebt)
			}
		})
	}
}

func TestScoringReadsRawBureauProtoJSON(t *testing.T) {
	profile, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(&bureaupb.CompanyProfileResponse{
		Status: bureaupb.CompanyStatus_COMPANY_STATUS_ACTIVE, RegisteredAt: "2019-04-15", Okved: "10.71",
	})
	if err != nil {
		t.Fatal(err)
	}
	zero := int32(0)
	credit, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(&bureaupb.CreditHistoryResponse{
		TotalDebt: &commonpb.Money{Currency: "RUB"}, OverdueCount_12M: &zero, MaxOverdueDays: &zero,
	})
	if err != nil {
		t.Fatal(err)
	}
	input, err := decodeSavedInput(
		uuid.MustParse("10000000-0000-4000-8000-000000000006"),
		uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		[]byte(`{"revenue_12m":{"amount_kopecks":840000000,"currency":"RUB"},"profit_12m":{"amount_kopecks":96000000,"currency":"RUB"}}`),
		profile, credit,
	)
	if err != nil {
		t.Fatal(err)
	}
	input.AsOfDate = time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	request, err := scoringclient.BuildRequest(input)
	if err != nil || request.Features.TotalDebt == nil || request.Features.OverdueCount_12M == nil ||
		*request.Features.OverdueCount_12M != 0 {
		t.Fatalf("raw bureau response was not usable for scoring: request=%v err=%v", request, err)
	}
}
