package gethandler

import (
	"encoding/json"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	"github.com/google/uuid"
)

func TestToBankCaseV2PreservesAbsentAndZeroMoney(t *testing.T) {
	version := int32(2)
	date := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	company := &commonpb.CompanySnapshot{Inn: "990000000115", Name: "ИП Демо", Okved: "47.11", RegisteredAt: "2020-01-01", ProfileSchemaVersion: &version, LegalForm: commonpb.LegalForm_LEGAL_FORM_IP, Revenue_12M: &commonpb.Money{AmountKopecks: 0, Currency: "RUB"}, TaxPeriods: []*commonpb.TaxPeriod{{From: "2020-01-01", Regimes: []commonpb.TaxRegime{commonpb.TaxRegime_TAX_REGIME_USN_INCOME}, VatStatus: commonpb.VatStatus_VAT_STATUS_UNKNOWN}}}
	plan := &commonpb.RequirementsPlan{PolicyVersion: "sme-profile-v1", ReferenceDate: "2026-10-26", CanSubmit: true, Route: commonpb.RequirementsRoute_REQUIREMENTS_ROUTE_STANDARD, Items: []*commonpb.Requirement{{Id: "company_profit_12m", Code: "COMPANY_PROFIT_12M", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_FIELD, RequiredAt: commonpb.RequirementStage_REQUIREMENT_STAGE_SUBMIT, Status: commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING, FieldPath: "company.profit_12m"}}}
	item := model.Case{CompanyV2: company, RequirementsPlan: plan, Documents: []model.DocumentMetadata{{ID: uuid.New(), DocumentType: "BANK_STATEMENT", UploadedAt: date, RequirementID: "cash_flow:2025-10-01:2026-09-30:all", CoveragePeriod: &commonpb.DatePeriod{From: "2025-10-01", To: "2026-09-30"}}}}
	got := ToBankCase(getcase.Output{Case: item})
	if got.Company.Revenue12m == nil || got.Company.Revenue12m.AmountKopecks != 0 || got.Company.Profit12m != nil || got.Company.Obligations != nil {
		t.Fatalf("money presence: %+v", got.Company)
	}
	if got.Company.LegalForm == nil || string(*got.Company.LegalForm) != "IP" || got.Company.TaxPeriods == nil || len(*got.Company.TaxPeriods) != 1 {
		t.Fatalf("profile: %+v", got.Company)
	}
	if got.RequirementsPlan == nil || got.RequirementsPlan.CanSubmit || len(got.RequirementsPlan.Items) != 1 || got.RequirementsPlan.Items[0].Id != "company_profit_12m" {
		t.Fatalf("plan: %+v", got.RequirementsPlan)
	}
	if got.Documents[0].RequirementId == nil || got.Documents[0].CoveragePeriod == nil {
		t.Fatalf("document: %+v", got.Documents[0])
	}
}

func TestToBankCaseIncludesDocumentMetadata(t *testing.T) {
	documentID := uuid.New()
	uploadedAt := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	caseItem := ToBankCase(getcase.Output{Case: model.Case{Documents: []model.DocumentMetadata{{
		ID: documentID, DocumentType: "BANK_STATEMENT", FileName: "example.txt",
		SHA256:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		UploadedAt: uploadedAt, IsMock: true,
	}}}})
	if len(caseItem.Documents) != 1 || caseItem.Documents[0].Id != documentID ||
		caseItem.Documents[0].FileName != "example.txt" || !caseItem.Documents[0].IsMock ||
		!caseItem.Documents[0].UploadedAt.Equal(uploadedAt) {
		t.Fatalf("case documents = %+v", caseItem.Documents)
	}
}

func TestToScoreKeepsZeroAsAssessed(t *testing.T) {
	zero := 0
	result := toScore(model.ScoreResult{
		AssessmentStatus:     "ASSESSED",
		Score:                &zero,
		Grade:                model.ScoreGrade("E"),
		ModelVersion:         "monotone-scorecard-v1",
		FeatureSchemaVersion: "borrower-features-v1",
	})

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal score: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("unmarshal score: %v", err)
	}
	if body["assessment_status"] != "ASSESSED" || body["score"] != float64(0) {
		t.Fatalf("score response = %s, want ASSESSED with present score 0", encoded)
	}
	if _, ok := body["data_issues"].([]any); !ok {
		t.Fatalf("score response = %s, want data_issues array", encoded)
	}
}

func TestToScoreKeepsUnassessedScoreAbsent(t *testing.T) {
	result := toScore(model.ScoreResult{
		AssessmentStatus:     "INSUFFICIENT_DATA",
		Grade:                "UNSPECIFIED",
		ModelVersion:         "monotone-scorecard-v1",
		FeatureSchemaVersion: "borrower-features-v1",
		DataIssues: []model.ScoreDataIssue{{
			Code: "MISSING_VALUE", FieldPath: "features.total_debt",
		}},
	})
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	if body["assessment_status"] != "INSUFFICIENT_DATA" || body["grade"] != "UNSPECIFIED" {
		t.Fatalf("score response = %s", encoded)
	}
	if _, ok := body["score"]; ok {
		t.Fatalf("unassessed response exposed score: %s", encoded)
	}
	if issues, ok := body["data_issues"].([]any); !ok || len(issues) != 1 {
		t.Fatalf("unassessed response lost issue: %s", encoded)
	}
}
