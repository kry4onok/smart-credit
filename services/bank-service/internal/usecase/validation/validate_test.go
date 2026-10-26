package validation

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
)

func completeSnapshot() Snapshot {
	return Snapshot{
		CompanyJSON:   []byte(`{"inn":"7707083893","name":"Test","registered_at":"2019-04-15","revenue_12m":{"amount_kopecks":"1000000","currency":"RUB"},"profit_12m":{"amount_kopecks":"0","currency":"RUB"},"obligations":{"amount_kopecks":"0","currency":"RUB"}}`),
		DocumentsJSON: []byte(`[{"document_id":"40000000-0000-4000-8000-000000000001","document_type":"BANK_STATEMENT","file_name":"statement.pdf","sha256":"` + strings.Repeat("a", 64) + `"}]`),
		AmountKopecks: 300000, TermMonths: 12, PurposeTag: "WORKING_CAPITAL",
	}
}

func testProduct() Product {
	return Product{
		Found: true, PurposeTags: []string{"WORKING_CAPITAL"},
		RequiredDocumentTypes: []string{"BANK_STATEMENT"},
	}
}

func TestEvaluateCompleteSnapshot(t *testing.T) {
	result := Evaluate(completeSnapshot(), testProduct())
	if result.Status != model.CaseStatusEnrichment || result.Reason != "" {
		t.Fatalf("outcome = %+v", result)
	}
}

func TestEvaluateRequestsMissingMoneyAndDocument(t *testing.T) {
	snapshot := completeSnapshot()
	snapshot.CompanyJSON = []byte(`{"inn":"7707083893","name":"Test","registered_at":"2019-04-15","profit_12m":{"amount_kopecks":"0","currency":"RUB"},"obligations":{"amount_kopecks":"0","currency":"RUB"}}`)
	snapshot.DocumentsJSON = []byte(`[]`)
	result := Evaluate(snapshot, testProduct())
	if result.Status != model.CaseStatusNeedsInfo || result.Reason != "MISSING_REQUIRED_DATA" ||
		!reflect.DeepEqual(result.RequiredFields, []string{"COMPANY_REVENUE_12M"}) ||
		!reflect.DeepEqual(result.RequiredDocumentTypes, []string{"BANK_STATEMENT"}) {
		t.Fatalf("outcome = %+v", result)
	}
}

func TestEvaluateRejectsIncompleteDocumentMetadata(t *testing.T) {
	snapshot := completeSnapshot()
	snapshot.DocumentsJSON = []byte(`[{"document_id":"40000000-0000-4000-8000-000000000001","document_type":"BANK_STATEMENT","file_name":"statement.pdf","sha256":"short"}]`)
	result := Evaluate(snapshot, testProduct())
	if result.Status != model.CaseStatusNeedsInfo ||
		!reflect.DeepEqual(result.RequiredDocumentTypes, []string{"BANK_STATEMENT"}) {
		t.Fatalf("outcome = %+v", result)
	}
}

func TestEvaluateRequestsPurposeCorrection(t *testing.T) {
	snapshot := completeSnapshot()
	snapshot.PurposeTag = "EQUIPMENT"
	result := Evaluate(snapshot, testProduct())
	if result.Status != model.CaseStatusNeedsInfo || !reflect.DeepEqual(result.RequiredFields,
		[]string{"APPLICATION_PURPOSE"}) {
		t.Fatalf("outcome = %+v", result)
	}
}

func TestEvaluateLeavesProductLimitsForDecision(t *testing.T) {
	snapshot := completeSnapshot()
	snapshot.AmountKopecks = 600000000000
	snapshot.TermMonths = 120
	if result := Evaluate(snapshot, testProduct()); result.Status != model.CaseStatusEnrichment {
		t.Fatalf("outcome = %+v", result)
	}
}

func TestEvaluateUnknownProductAndMalformedSnapshot(t *testing.T) {
	if result := Evaluate(completeSnapshot(), Product{}); result.Status != model.CaseStatusManualReview ||
		result.Reason != "UNKNOWN_PRODUCT" {
		t.Fatalf("unknown product outcome = %+v", result)
	}
	snapshot := completeSnapshot()
	snapshot.CompanyJSON = []byte(`not-json`)
	if result := Evaluate(snapshot, testProduct()); result.Status != model.CaseStatusManualReview ||
		result.Reason != "VALIDATION_INVALID_SNAPSHOT" {
		t.Fatalf("malformed snapshot outcome = %+v", result)
	}
}

func TestLegacyProductKeepsLegacyPolicyWithV2Profile(t *testing.T) {
	snapshot := completeSnapshot()
	snapshot.CompanyJSON = []byte(strings.Replace(string(snapshot.CompanyJSON), `"inn":`, `"profile_schema_version":2,"inn":`, 1))
	result := Evaluate(snapshot, testProduct())
	if result.Status != model.CaseStatusEnrichment || result.Plan != nil {
		t.Fatalf("unexpected legacy processing: %+v", result)
	}
}
