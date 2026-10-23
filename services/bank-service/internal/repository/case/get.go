package caserepository

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
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
            THEN replace(company_check.result->>'status', 'COMPANY_STATUS_', '')
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
    current_attempt.id,
    current_attempt.input_revision,
    current_attempt.as_of_date,
    current_attempt.status,
    current_attempt.response,
    cases.triggered_rules,
    cases.recommendation,
    cases.assignee_id,
    active_info_request.id,
    active_info_request.message,
    active_info_request.required_fields,
    active_info_request.required_document_types,
    active_info_request.created_at,
    active_offer.id,
    active_offer.kind,
    active_offer.product_id,
    active_offer.product_version,
    active_offer.product_name,
    active_offer.amount_kopecks,
    active_offer.term_months,
    active_offer.rate_bps,
    active_offer.expires_at,
    active_offer.explanation,
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
    WHERE case_id = cases.id AND attempt_id IS NULL
        AND cases.current_scoring_attempt_id IS NULL
        AND cases.scoring_input_revision = 1
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) AS latest_score ON true
LEFT JOIN scoring_attempts AS current_attempt
    ON current_attempt.id = cases.current_scoring_attempt_id
    AND current_attempt.input_revision = cases.scoring_input_revision
LEFT JOIN LATERAL (
    SELECT id, message, required_fields, required_document_types, created_at
    FROM info_requests
    WHERE case_id = cases.id AND status = 'OPEN'
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) AS active_info_request ON true
LEFT JOIN LATERAL (
    SELECT
        id,
        kind,
        product_id,
        product_version,
        product_name,
        amount_kopecks,
        term_months,
        rate_bps,
        expires_at,
        explanation
    FROM offers
    WHERE case_id = cases.id
        AND status = 'AVAILABLE'
        AND expires_at > now()
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) AS active_offer ON true
WHERE cases.id = $1`

func (r *Repository) GetCase(
	ctx context.Context,
	caseID uuid.UUID,
) (model.Case, bool, error) {
	caseItem, err := scanCase(transaction.QueryerFromContext(ctx, r.db).QueryRow(ctx, getCaseQuery, caseID))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Case{}, false, nil
	}
	if err != nil {
		return model.Case{}, false, fmt.Errorf("scan case: %w", err)
	}

	return caseItem, true, nil
}
