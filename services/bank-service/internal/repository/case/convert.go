package caserepository

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	scoringpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/scoring"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

func scanCaseListItem(row rowScanner) (model.CaseListItem, error) {
	var item model.CaseListItem
	var status string
	var assigneeID *uuid.UUID

	err := row.Scan(
		&item.ID,
		&status,
		&item.ProductID,
		&item.RequestedAmountKopecks,
		&item.CompanyName,
		&assigneeID,
		&item.CreatedAt,
		&item.Version,
	)
	if err != nil {
		return model.CaseListItem{}, err
	}

	item.Status = model.CaseStatus(status)
	item.AssigneeID = assigneeID

	return item, nil
}

func scanCase(row rowScanner) (model.Case, error) {
	var item model.Case
	var status string
	var purpose string
	var documentTypes []string
	var documentsJSON []byte
	var hasChecks bool
	var companyStatus string
	var registeredAt *time.Time
	var okved *string
	var totalDebtKopecks int64
	var overdueCount12M int
	var maxOverdueDays int
	var bureauError *string
	var score *int
	var grade *string
	var reasonsJSON []byte
	var modelVersion *string
	var featureSchemaVersion *string
	var currentAttemptID *uuid.UUID
	var currentInputRevision *int64
	var currentAsOfDate *time.Time
	var currentAttemptStatus *string
	var currentResponseJSON []byte
	var recommendation *string
	var infoRequestID *uuid.UUID
	var infoRequestMessage *string
	var requiredFields []string
	var requiredDocumentTypes []string
	var infoRequestCreatedAt *time.Time
	var offerID *uuid.UUID
	var offerKind *string
	var offerProductID *uuid.UUID
	var offerProductVersion *int64
	var offerProductName *string
	var offerAmountKopecks *int64
	var offerTermMonths *int
	var offerRateBPS *int
	var offerExpiresAt *time.Time
	var offerExplanation *string
	var companyJSON []byte
	var referenceDate *time.Time
	var policyVersion string
	var productFound bool
	var productPurposeTags, productDocumentTypes []string
	var productMinAmount, productMaxAmount int64
	var productMinTerm, productMaxTerm int32
	var activeRequirementsJSON []byte

	err := row.Scan(
		&item.ID,
		&status,
		&item.ProductID,
		&item.ProductName,
		&item.ProductVersion,
		&item.Company.INN,
		&item.Company.Name,
		&item.Company.OKVED,
		&item.Company.RegisteredAt,
		&item.Company.Revenue12MKopecks,
		&item.Company.Profit12MKopecks,
		&item.Company.ObligationsKopecks,
		&item.Application.AmountKopecks,
		&item.Application.TermMonths,
		&purpose,
		&item.Application.PurposeDescription,
		&documentTypes,
		&documentsJSON,
		&hasChecks,
		&companyStatus,
		&registeredAt,
		&okved,
		&totalDebtKopecks,
		&overdueCount12M,
		&maxOverdueDays,
		&bureauError,
		&score,
		&grade,
		&reasonsJSON,
		&modelVersion,
		&featureSchemaVersion,
		&currentAttemptID,
		&currentInputRevision,
		&currentAsOfDate,
		&currentAttemptStatus,
		&currentResponseJSON,
		&item.TriggeredRules,
		&recommendation,
		&item.AssigneeID,
		&infoRequestID,
		&infoRequestMessage,
		&requiredFields,
		&requiredDocumentTypes,
		&infoRequestCreatedAt,
		&offerID,
		&offerKind,
		&offerProductID,
		&offerProductVersion,
		&offerProductName,
		&offerAmountKopecks,
		&offerTermMonths,
		&offerRateBPS,
		&offerExpiresAt,
		&offerExplanation,
		&item.PublicReason,
		&item.AuditVersion,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
		&companyJSON,
		&referenceDate,
		&policyVersion,
		&productFound,
		&productPurposeTags,
		&productDocumentTypes,
		&productMinAmount,
		&productMaxAmount,
		&productMinTerm,
		&productMaxTerm,
		&activeRequirementsJSON,
	)
	if err != nil {
		return model.Case{}, err
	}

	item.Status = model.CaseStatus(status)
	item.Application.Purpose = model.PurposeTag(purpose)
	item.Application.DocumentTypes = make([]model.DocumentType, len(documentTypes))
	for index, documentType := range documentTypes {
		item.Application.DocumentTypes[index] = model.DocumentType(documentType)
	}
	item.Documents, err = decodeCaseDocuments(documentsJSON)
	if err != nil {
		return model.Case{}, fmt.Errorf("decode case documents: %w", err)
	}
	if hasChecks {
		item.Checks = &model.Checks{
			CompanyStatus:    model.CompanyStatus(companyStatus),
			RegisteredAt:     registeredAt,
			OKVED:            okved,
			TotalDebtKopecks: totalDebtKopecks,
			OverdueCount12M:  overdueCount12M,
			MaxOverdueDays:   maxOverdueDays,
			BureauError:      bureauError,
		}
	}
	if score != nil {
		reasons, decodeErr := decodeScoreReasons(reasonsJSON)
		if decodeErr != nil {
			return model.Case{}, fmt.Errorf("decode score reasons: %w", decodeErr)
		}
		item.Score = &model.ScoreResult{
			AssessmentStatus:     "ASSESSED",
			Score:                score,
			Grade:                model.ScoreGrade(*grade),
			Reasons:              reasons,
			ModelVersion:         *modelVersion,
			FeatureSchemaVersion: *featureSchemaVersion,
		}
	}
	if currentAttemptID != nil {
		if currentAttemptStatus == nil {
			return model.Case{}, fmt.Errorf("current scoring attempt has no status")
		}

		if *currentAttemptStatus == "ASSESSED" || *currentAttemptStatus == "INSUFFICIENT_DATA" ||
			*currentAttemptStatus == "UNSUPPORTED_INPUT" {
			currentScore, decodeErr := decodeCurrentScore(
				currentResponseJSON, *currentAttemptID, *currentInputRevision, *currentAsOfDate,
			)
			if decodeErr != nil {
				return model.Case{}, fmt.Errorf("decode current scoring attempt: %w", decodeErr)
			}
			item.Score = currentScore
		} else {
			item.Score = nil
		}
	}
	if recommendation != nil {
		value := model.CaseRecommendation(*recommendation)
		item.Recommendation = &value
	}
	if infoRequestID != nil {
		item.InfoRequest = &model.InfoRequest{
			ID:                    *infoRequestID,
			Message:               *infoRequestMessage,
			RequiredFields:        convertInfoFields(requiredFields),
			RequiredDocumentTypes: convertDocumentTypes(requiredDocumentTypes),
			CreatedAt:             *infoRequestCreatedAt,
		}
		item.InfoRequest.Requirements, err = decodeRequirementsJSON(activeRequirementsJSON)
		if err != nil {
			return model.Case{}, fmt.Errorf("decode active requirements: %w", err)
		}
	}
	if offerID != nil {
		item.Offer = &model.Offer{
			ID:             *offerID,
			Kind:           model.OfferKind(*offerKind),
			ProductID:      *offerProductID,
			ProductVersion: *offerProductVersion,
			ProductName:    *offerProductName,
			AmountKopecks:  *offerAmountKopecks,
			TermMonths:     *offerTermMonths,
			RateBPS:        *offerRateBPS,
			ExpiresAt:      *offerExpiresAt,
			Explanation:    *offerExplanation,
		}
	}

	companyV2 := new(commonpb.CompanySnapshot)
	if err := protojson.Unmarshal(companyJSON, companyV2); err == nil && companyV2.GetProfileSchemaVersion() == 2 {
		item.CompanyV2 = companyV2
	}
	if policyVersion != "" {
		outcome := validation.Evaluate(validation.Snapshot{CompanyJSON: companyJSON, DocumentsJSON: documentsJSON, ReferenceDate: referenceDate, AmountKopecks: item.Application.AmountKopecks, TermMonths: int32(item.Application.TermMonths), PurposeTag: purpose, PurposeDescription: item.Application.PurposeDescription}, validation.Product{Found: productFound, PolicyVersion: policyVersion, PurposeTags: productPurposeTags, RequiredDocumentTypes: productDocumentTypes, MinAmountKopecks: productMinAmount, MaxAmountKopecks: productMaxAmount, MinTermMonths: productMinTerm, MaxTermMonths: productMaxTerm})
		item.RequirementsPlan = outcome.Plan
	}
	return item, nil
}

func decodeRequirementsJSON(data []byte) ([]*commonpb.Requirement, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	items := make([]*commonpb.Requirement, 0, len(raw))
	for _, part := range raw {
		item := new(commonpb.Requirement)
		if err := protojson.Unmarshal(part, item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func decodeCaseDocuments(raw []byte) ([]model.DocumentMetadata, error) {
	var stored []struct {
		DocumentID     uuid.UUID            `json:"document_id"`
		LegacyID       uuid.UUID            `json:"id"`
		DocumentType   string               `json:"document_type"`
		FileName       string               `json:"file_name"`
		SHA256         string               `json:"sha256"`
		UploadedAt     time.Time            `json:"uploaded_at"`
		IsMock         bool                 `json:"is_mock"`
		RequirementID  string               `json:"requirement_id"`
		CoveragePeriod *commonpb.DatePeriod `json:"coverage_period"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	documents := make([]model.DocumentMetadata, 0, len(stored))
	for _, document := range stored {
		id := document.DocumentID
		if id == uuid.Nil {
			id = document.LegacyID
		}
		if id == uuid.Nil {
			return nil, fmt.Errorf("missing document id")
		}
		documents = append(documents, model.DocumentMetadata{
			ID: id, DocumentType: document.DocumentType, FileName: document.FileName,
			SHA256: document.SHA256, UploadedAt: document.UploadedAt, IsMock: document.IsMock,
			RequirementID: document.RequirementID, CoveragePeriod: document.CoveragePeriod,
		})
	}
	return documents, nil
}

func decodeCurrentScore(data []byte, attemptID uuid.UUID, revision int64, asOfDate time.Time) (*model.ScoreResult, error) {
	response := &scoringpb.ScoreResponse{}
	if err := protojson.Unmarshal(data, response); err != nil {
		return nil, err
	}

	status := strings.TrimPrefix(response.AssessmentStatus.String(), "ASSESSMENT_STATUS_")
	if status != "ASSESSED" && status != "INSUFFICIENT_DATA" && status != "UNSUPPORTED_INPUT" {
		return nil, fmt.Errorf("invalid scoring status %q", status)
	}
	result := &model.ScoreResult{
		AssessmentStatus:     status,
		Grade:                model.ScoreGrade(strings.TrimPrefix(response.Grade.String(), "GRADE_")),
		ModelVersion:         response.ModelVersion,
		FeatureSchemaVersion: response.FeatureSchemaVersion,
		AttemptID:            &attemptID,
		InputRevision:        &revision,
		AsOfDate:             &asOfDate,
		DataIssues:           make([]model.ScoreDataIssue, 0, len(response.DataIssues)),
		Reasons:              make([]model.ScoreReason, 0, len(response.Reasons)),
		FactorContributions:  make([]model.ScoreFactorContribution, 0, len(response.FactorContributions)),
	}
	if response.Score != nil {
		value := int(*response.Score)
		result.Score = &value
	}

	baseline := int(response.BaselineScore)
	result.BaselineScore = &baseline
	checksum := response.ConfigChecksum
	result.ConfigChecksum = &checksum
	for _, issue := range response.DataIssues {
		result.DataIssues = append(result.DataIssues, model.ScoreDataIssue{Code: issue.Code, FieldPath: issue.FieldPath})
	}
	for _, factor := range response.FactorContributions {
		result.FactorContributions = append(result.FactorContributions, model.ScoreFactorContribution{
			FactorName: factor.FactorName, Points: int(factor.Points), Description: factor.Description,
		})
	}
	for _, reason := range response.Reasons {
		result.Reasons = append(result.Reasons, model.ScoreReason{
			Code:         model.ScoreReasonCode(strings.TrimPrefix(reason.Code.String(), "REASON_CODE_")),
			Description:  reason.Description,
			Direction:    model.ScoreReasonDirection(strings.TrimPrefix(reason.Direction.String(), "DIRECTION_")),
			Contribution: int(reason.Contribution),
		})
	}
	return result, nil
}

func convertInfoFields(values []string) []model.InfoField {
	result := make([]model.InfoField, len(values))
	for index, value := range values {
		result[index] = model.InfoField(value)
	}

	return result
}

func convertDocumentTypes(values []string) []model.DocumentType {
	result := make([]model.DocumentType, len(values))
	for index, value := range values {
		result[index] = model.DocumentType(value)
	}

	return result
}

type scoreReasonRecord struct {
	Code         string `json:"code"`
	Description  string `json:"description"`
	Direction    string `json:"direction"`
	Contribution int    `json:"contribution"`
}

func decodeScoreReasons(data []byte) ([]model.ScoreReason, error) {
	var records []scoreReasonRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}

	reasons := make([]model.ScoreReason, 0, len(records))
	for _, record := range records {
		reasons = append(reasons, model.ScoreReason{
			Code:         model.ScoreReasonCode(record.Code),
			Description:  record.Description,
			Direction:    model.ScoreReasonDirection(record.Direction),
			Contribution: record.Contribution,
		})
	}

	return reasons, nil
}
