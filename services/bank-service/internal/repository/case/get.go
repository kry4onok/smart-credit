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
    cases.id,
    cases.status,
    cases.product_id,
    cases.product_name,
    cases.product_version,
    cases.company_snapshot->>'inn',
    cases.company_snapshot->>'name',
    cases.company_snapshot->>'okved',
    (cases.company_snapshot->>'registered_at')::date,
    (cases.company_snapshot->'revenue_12m'->>'amount_kopecks')::bigint,
    (cases.company_snapshot->'profit_12m'->>'amount_kopecks')::bigint,
    (cases.company_snapshot->'obligations'->>'amount_kopecks')::bigint,
    cases.requested_amount_kopecks,
    cases.requested_term_months,
    cases.purpose_tag,
    COALESCE((
        SELECT array_agg(document->>'document_type' ORDER BY ordinality)
        FROM jsonb_array_elements(cases.documents_snapshot)
            WITH ORDINALITY AS documents(document, ordinality)
    ), ARRAY[]::text[]),
    company_check.id IS NOT NULL OR credit_check.id IS NOT NULL,
    COALESCE(
        CASE WHEN company_check.outcome = 'SUCCESS'
            THEN company_check.result->>'status'
        END,
        'UNKNOWN'
    ),
    CASE WHEN company_check.outcome = 'SUCCESS'
        THEN (company_check.result->>'registered_at')::date
    END,
    CASE WHEN company_check.outcome = 'SUCCESS'
        THEN company_check.result->>'okved'
    END,
    COALESCE(
        CASE WHEN credit_check.outcome = 'SUCCESS'
            THEN (credit_check.result->'total_debt'->>'amount_kopecks')::bigint
        END,
        0
    ),
    COALESCE(
        CASE WHEN credit_check.outcome = 'SUCCESS'
            THEN (credit_check.result->>'overdue_count_12m')::integer
        END,
        0
    ),
    COALESCE(
        CASE WHEN credit_check.outcome = 'SUCCESS'
            THEN (credit_check.result->>'max_overdue_days')::integer
        END,
        0
    ),
    NULLIF(concat_ws(', ', company_check.error_code, credit_check.error_code), ''),
    latest_score.score,
    latest_score.grade,
    latest_score.reasons,
    latest_score.model_version,
    latest_score.feature_schema_version,
    cases.triggered_rules,
    cases.recommendation,
    cases.assignee_id,
    cases.public_reason,
    cases.audit_version,
    cases.created_at,
    cases.updated_at,
    cases.version
FROM cases
LEFT JOIN LATERAL (
    SELECT id, outcome, result, error_code
    FROM enrichment_results
    WHERE case_id = cases.id AND source = 'COMPANY'
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) AS company_check ON true
LEFT JOIN LATERAL (
    SELECT id, outcome, result, error_code
    FROM enrichment_results
    WHERE case_id = cases.id AND source = 'CREDIT_HISTORY'
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) AS credit_check ON true
LEFT JOIN LATERAL (
    SELECT score, grade, reasons, model_version, feature_schema_version
    FROM score_results
    WHERE case_id = cases.id
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) AS latest_score ON true
WHERE cases.id = $1`

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
