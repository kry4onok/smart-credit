package caserepository

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
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
	var recommendation *string

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
		&documentTypes,
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
		&item.TriggeredRules,
		&recommendation,
		&item.AssigneeID,
		&item.PublicReason,
		&item.AuditVersion,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
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
			Score:                *score,
			Grade:                model.ScoreGrade(*grade),
			Reasons:              reasons,
			ModelVersion:         *modelVersion,
			FeatureSchemaVersion: *featureSchemaVersion,
		}
	}
	if recommendation != nil {
		value := model.CaseRecommendation(*recommendation)
		item.Recommendation = &value
	}

	return item, nil
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
