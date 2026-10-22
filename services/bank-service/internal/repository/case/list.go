package caserepository

import (
	"context"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
	"github.com/google/uuid"
)

const listCasesQuery = `
SELECT
    id,
    status,
    product_id,
    requested_amount_kopecks,
    company_name,
    assignee_id,
    created_at,
    version
FROM cases
WHERE ($1::text IS NULL OR status = $1)
  AND ($2::uuid IS NULL OR product_id = $2)
  AND (
      $3::smallint = 0
      OR ($3 = 1 AND assignee_id IS NULL)
      OR ($3 = 2 AND assignee_id = $4::uuid)
  )
  AND (
      $5::boolean IS NULL
      OR CASE WHEN status = 'MANUAL_REVIEW' THEN 0 ELSE 1 END
          > CASE WHEN $5::boolean THEN 0 ELSE 1 END
      OR (
          CASE WHEN status = 'MANUAL_REVIEW' THEN 0 ELSE 1 END
              = CASE WHEN $5::boolean THEN 0 ELSE 1 END
          AND (created_at, id) > ($6::timestamptz, $7::uuid)
      )
  )
ORDER BY
    CASE WHEN status = 'MANUAL_REVIEW' THEN 0 ELSE 1 END,
    created_at,
    id
LIMIT $8`

func (r *Repository) ListCases(
	ctx context.Context,
	query listcase.RepositoryQuery,
) ([]model.CaseListItem, error) {
	var status *string
	if query.Status != nil {
		value := string(*query.Status)
		status = &value
	}

	var assigneeID *uuid.UUID
	if query.Assignee.Kind == listcase.AssigneeUser {
		assigneeID = &query.Assignee.ID
	}

	var cursorManualReview *bool
	var cursorCreatedAt *time.Time
	var cursorID *uuid.UUID
	if query.After != nil {
		cursorManualReview = &query.After.ManualReview
		cursorCreatedAt = &query.After.CreatedAt
		cursorID = &query.After.ID
	}

	rows, err := transaction.QueryerFromContext(ctx, r.db).Query(
		ctx,
		listCasesQuery,
		status,
		query.ProductID,
		int16(query.Assignee.Kind),
		assigneeID,
		cursorManualReview,
		cursorCreatedAt,
		cursorID,
		query.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query cases: %w", err)
	}
	defer rows.Close()

	items := make([]model.CaseListItem, 0, query.Limit)
	for rows.Next() {
		item, scanErr := scanCaseListItem(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan case: %w", scanErr)
		}

		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cases: %w", err)
	}

	return items, nil
}
