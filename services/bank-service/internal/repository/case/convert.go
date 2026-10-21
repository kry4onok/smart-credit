package caserepository

import (
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
	if recommendation != nil {
		value := model.CaseRecommendation(*recommendation)
		item.Recommendation = &value
	}

	return item, nil
}
