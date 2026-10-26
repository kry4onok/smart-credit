package product

import (
	"context"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func (r *Repository) List(ctx context.Context, purpose *string) (model.ProductCatalog, error) {
	var version int64
	if err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(aggregate_version), 1)
		FROM processed_events
		WHERE topic = 'bank.events' AND aggregate_id = 'catalog'`).Scan(&version); err != nil {
		return model.ProductCatalog{}, fmt.Errorf("fetch catalog version: %w", err)
	}

	var purposeArg any
	if purpose != nil {
		purposeArg = *purpose
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, version, code, name, description, purpose_tags,
		       min_amount_kopecks, max_amount_kopecks, min_term_months,
		       max_term_months, indicative_rate_bps, required_document_types, requirements_policy_version
		FROM products
		WHERE ($1::text IS NULL OR $1 = ANY(purpose_tags))
		ORDER BY code, id`, purposeArg)
	if err != nil {
		return model.ProductCatalog{}, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	items := make([]model.Product, 0)
	for rows.Next() {
		var item model.Product
		if err := rows.Scan(
			&item.ID, &item.Version, &item.Code, &item.Name, &item.Description,
			&item.PurposeTags, &item.MinAmountKopecks, &item.MaxAmountKopecks,
			&item.MinTermMonths, &item.MaxTermMonths, &item.IndicativeRateBps,
			&item.RequiredDocumentTypes, &item.RequirementsPolicyVersion,
		); err != nil {
			return model.ProductCatalog{}, fmt.Errorf("scan product: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return model.ProductCatalog{}, fmt.Errorf("iterate products: %w", err)
	}

	return model.ProductCatalog{Items: items, Version: version}, nil
}
