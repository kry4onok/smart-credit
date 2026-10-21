package caserepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const getCaseQuery = `
SELECT
    id,
    status,
    product_id,
    product_name,
    product_version,
    company_snapshot->>'inn',
    company_snapshot->>'name',
    company_snapshot->>'okved',
    (company_snapshot->>'registered_at')::date,
    (company_snapshot->'revenue_12m'->>'amount_kopecks')::bigint,
    (company_snapshot->'profit_12m'->>'amount_kopecks')::bigint,
    (company_snapshot->'obligations'->>'amount_kopecks')::bigint,
    requested_amount_kopecks,
    requested_term_months,
    purpose_tag,
    COALESCE((
        SELECT array_agg(document->>'document_type' ORDER BY ordinality)
        FROM jsonb_array_elements(documents_snapshot)
            WITH ORDINALITY AS documents(document, ordinality)
    ), ARRAY[]::text[]),
    triggered_rules,
    recommendation,
    assignee_id,
    public_reason,
    audit_version,
    created_at,
    updated_at,
    version
FROM cases
WHERE id = $1`

func (r *Repository) GetCase(
	ctx context.Context,
	caseID uuid.UUID,
) (model.Case, bool, error) {
	caseItem, err := scanCase(r.db.QueryRow(ctx, getCaseQuery, caseID))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Case{}, false, nil
	}
	if err != nil {
		return model.Case{}, false, fmt.Errorf("scan case: %w", err)
	}

	return caseItem, true, nil
}
