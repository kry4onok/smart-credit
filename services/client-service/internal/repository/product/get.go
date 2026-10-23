package product

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (model.Product, error) {
	var item model.Product
	err := r.db.QueryRow(ctx, `
		SELECT id, version, code, name, description, purpose_tags,
		       min_amount_kopecks, max_amount_kopecks, min_term_months,
		       max_term_months, indicative_rate_bps, required_document_types
		FROM products
		WHERE id = $1`, id).Scan(
		&item.ID, &item.Version, &item.Code, &item.Name, &item.Description,
		&item.PurposeTags, &item.MinAmountKopecks, &item.MaxAmountKopecks,
		&item.MinTermMonths, &item.MaxTermMonths, &item.IndicativeRateBps,
		&item.RequiredDocumentTypes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Product{}, domain.ErrProductNotFound
	}
	if err != nil {
		return model.Product{}, fmt.Errorf("fetch product: %w", err)
	}
	return item, nil
}
