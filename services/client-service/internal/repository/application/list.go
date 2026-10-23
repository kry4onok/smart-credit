package application

import (
	"context"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
	"github.com/google/uuid"
)

func (r *Repository) List(ctx context.Context, query listapplication.RepositoryQuery) ([]model.ApplicationListItem, error) {
	var status *string
	if query.Status != nil {
		status = query.Status
	}
	var afterCreatedAt *time.Time
	var afterID *uuid.UUID
	if query.After != nil {
		afterCreatedAt = &query.After.CreatedAt
		afterID = &query.After.ID
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, product_id, requested_amount_kopecks, status, next_action, created_at
		FROM applications
		WHERE org_id = $1
		  AND ($2::text IS NULL OR status = $2)
		  AND ($3::timestamptz IS NULL OR (created_at, id) > ($3::timestamptz, $4::uuid))
		ORDER BY created_at, id
		LIMIT $5`, query.OrgID, status, afterCreatedAt, afterID, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("query applications: %w", err)
	}
	defer rows.Close()
	items := make([]model.ApplicationListItem, 0, query.Limit)
	for rows.Next() {
		var item model.ApplicationListItem
		if err := rows.Scan(&item.ID, &item.ProductID, &item.AmountKopecks, &item.Status, &item.NextAction, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applications: %w", err)
	}
	return items, nil
}
