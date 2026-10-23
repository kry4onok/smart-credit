package validation

import (
	"encoding/json"
	"strings"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"google.golang.org/protobuf/encoding/protojson"
)

type Snapshot struct {
	CompanyJSON   []byte
	DocumentsJSON []byte
	AmountKopecks int64
	TermMonths    int32
	PurposeTag    string
}

type Product struct {
	Found                 bool
	PurposeTags           []string
	RequiredDocumentTypes []string
}

type Outcome struct {
	Status                model.CaseStatus
	Reason                string
	RequiredFields        []string
	RequiredDocumentTypes []string
}

func Evaluate(snapshot Snapshot, product Product) Outcome {
	if !product.Found {
		return Outcome{Status: model.CaseStatusManualReview, Reason: "UNKNOWN_PRODUCT"}
	}
	company := &commonpb.CompanySnapshot{}
	if err := protojson.Unmarshal(snapshot.CompanyJSON, company); err != nil ||
		strings.TrimSpace(company.GetRegisteredAt()) == "" {
		return Outcome{Status: model.CaseStatusManualReview, Reason: "VALIDATION_INVALID_SNAPSHOT"}
	}
	var documents []struct {
		DocumentType string `json:"document_type"`
		DocumentID   string `json:"document_id"`
		LegacyID     string `json:"id"`
		FileName     string `json:"file_name"`
		SHA256       string `json:"sha256"`
	}
	if err := json.Unmarshal(snapshot.DocumentsJSON, &documents); err != nil {
		return Outcome{Status: model.CaseStatusManualReview, Reason: "VALIDATION_INVALID_SNAPSHOT"}
	}
	result := Outcome{Status: model.CaseStatusEnrichment}
	for _, item := range []struct {
		missing bool
		field   string
	}{
		{company.GetRevenue_12M() == nil, "COMPANY_REVENUE_12M"},
		{company.GetProfit_12M() == nil, "COMPANY_PROFIT_12M"},
		{company.GetObligations() == nil, "COMPANY_OBLIGATIONS"},
		{snapshot.AmountKopecks <= 0, "APPLICATION_AMOUNT"},
		{snapshot.TermMonths <= 0, "APPLICATION_TERM_MONTHS"},
		{!contains(product.PurposeTags, snapshot.PurposeTag), "APPLICATION_PURPOSE"},
	} {
		if item.missing {
			result.RequiredFields = append(result.RequiredFields, item.field)
		}
	}
	for _, required := range product.RequiredDocumentTypes {
		present := false
		for _, document := range documents {
			if document.DocumentType == required &&
				(document.DocumentID != "" || document.LegacyID != "") &&
				strings.TrimSpace(document.FileName) != "" && len(document.SHA256) == 64 {
				present = true
				break
			}
		}
		if !present {
			result.RequiredDocumentTypes = append(result.RequiredDocumentTypes, required)
		}
	}
	if len(result.RequiredFields)+len(result.RequiredDocumentTypes) > 0 {
		result.Status = model.CaseStatusNeedsInfo
		result.Reason = "MISSING_REQUIRED_DATA"
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
