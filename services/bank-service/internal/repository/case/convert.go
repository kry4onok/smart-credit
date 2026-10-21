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
