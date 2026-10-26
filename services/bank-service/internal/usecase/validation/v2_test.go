package validation

import (
	"encoding/json"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"google.golang.org/protobuf/encoding/protojson"
)

func v2ValidationInput(t *testing.T) (Snapshot, Product, *commonpb.CompanySnapshot) {
	t.Helper()
	version := int32(2)
	ref := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	company := &commonpb.CompanySnapshot{Inn: "990000000115", Name: "ИП Демо", Okved: "47.11", RegisteredAt: "2020-01-01", ProfileSchemaVersion: &version, LegalForm: commonpb.LegalForm_LEGAL_FORM_IP, TaxPeriods: []*commonpb.TaxPeriod{{From: "2020-01-01", Regimes: []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_USN_INCOME}, VatStatus: commonpb.VatStatus_VAT_STATUS_UNKNOWN}}, Revenue_12M: &commonpb.Money{AmountKopecks: 100000000, Currency: "RUB"}, Profit_12M: &commonpb.Money{AmountKopecks: 0, Currency: "RUB"}, Obligations: &commonpb.Money{AmountKopecks: 0, Currency: "RUB"}, FinancialPeriod: &commonpb.DatePeriod{From: "2025-10-01", To: "2026-09-30"}, FinancialBasis: commonpb.FinancialBasis_FINANCIAL_BASIS_MANAGEMENT, ObligationsAsOf: "2026-10-20"}
	data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(company)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{CompanyJSON: data, DocumentsJSON: []byte(`[]`), ReferenceDate: &ref, AmountKopecks: 100000000, TermMonths: 24, PurposeTag: "WORKING_CAPITAL"}
	product := Product{Found: true, PolicyVersion: "sme-profile-v2", PurposeTags: []string{"WORKING_CAPITAL"}, MinAmountKopecks: 10000000, MaxAmountKopecks: 500000000, MinTermMonths: 6, MaxTermMonths: 36}
	return snapshot, product, company
}

func TestEvaluateV2DefersAssessmentDocumentsUntilSources(t *testing.T) {
	snapshot, product, _ := v2ValidationInput(t)
	outcome := Evaluate(snapshot, product)
	if outcome.Status != model.CaseStatusEnrichment || outcome.Plan == nil || len(outcome.RequestedRequirements) != 0 {
		t.Fatalf("outcome=%+v", outcome)
	}
	for _, item := range outcome.Plan.Items {
		if item.Status != commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING || item.RequiredAt != commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT {
			continue
		}
		if item.GetRequiredAt() != commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT || item.GetKind() != commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT {
			t.Fatalf("unexpected request: %v", item)
		}
	}
	if outcome.Plan.GetRoute() != commonpb.RequirementsRoute_REQUIREMENTS_ROUTE_STANDARD {
		t.Fatalf("route=%v", outcome.Plan.Route)
	}
	docs := make([]receive.DocumentSnapshot, 0, 2)
	for _, item := range outcome.Plan.Items {
		if item.Status != commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING || item.RequiredAt != commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT {
			continue
		}
		docs = append(docs, receive.DocumentSnapshot{DocumentID: item.Id, DocumentType: "BANK_STATEMENT", UploadedAt: *snapshot.ReferenceDate, RequirementID: item.Id, CoveragePeriod: item.Period})
		if item.Code == "BUSINESS_RESULT" {
			docs[len(docs)-1].DocumentType = "MANAGEMENT_REPORT"
		}
	}
	data, err := json.Marshal(docs)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.DocumentsJSON = data
	complete := Evaluate(snapshot, product)
	if complete.Status != model.CaseStatusEnrichment || len(complete.RequestedRequirements) != 0 {
		t.Fatalf("complete=%+v", complete)
	}
}

func TestEvaluateV2MissingMoneyAndManualExceptions(t *testing.T) {
	snapshot, product, company := v2ValidationInput(t)
	company.Profit_12M = nil
	data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(company)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.CompanyJSON = data
	outcome := Evaluate(snapshot, product)
	if outcome.Status != model.CaseStatusEnrichment {
		t.Fatalf("missing money=%+v", outcome)
	}
	if len(outcome.RequestedRequirements) != 0 {
		t.Fatalf("baseline profit must not block source collection: %+v", outcome)
	}
	product.PolicyVersion = "unknown"
	if got := Evaluate(snapshot, product); got.Status != model.CaseStatusManualReview || got.Reason != "UNKNOWN_REQUIREMENTS_POLICY" {
		t.Fatalf("unknown policy=%+v", got)
	}
	product.PolicyVersion = "sme-profile-v2"
	company.Profit_12M = &commonpb.Money{AmountKopecks: 0, Currency: "RUB"}
	company.TaxPeriods[0].Regimes = []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_AUSN_INCOME}
	data, err = (protojson.MarshalOptions{UseProtoNames: true}).Marshal(company)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.CompanyJSON = data
	if got := Evaluate(snapshot, product); got.Status != model.CaseStatusManualReview || got.Reason != "TAX_REGIME_REQUIRES_REVIEW" {
		t.Fatalf("manual regime=%+v", got)
	}
}
