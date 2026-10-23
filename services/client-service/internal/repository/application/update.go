package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) Update(ctx context.Context, params model.UpdateApplicationParams) (model.Application, error) {
	patch := params.Patch
	result, err := r.db.Exec(ctx, `
		UPDATE applications
		SET requested_amount_kopecks = CASE WHEN $3::boolean THEN $4::bigint ELSE requested_amount_kopecks END,
		    requested_term_months = CASE WHEN $5::boolean THEN $6::integer ELSE requested_term_months END,
		    purpose_tag = CASE WHEN $7::boolean THEN $8::text ELSE purpose_tag END,
		    version = version + 1,
		    updated_at = now()
		WHERE id = $1 AND org_id = $2 AND status = 'DRAFT' AND version = $9`,
		params.ApplicationID, params.OrgID,
		patch.AmountKopecks != nil, valueOrZero(patch.AmountKopecks),
		patch.TermMonths != nil, valueOrZero(patch.TermMonths),
		patch.Purpose != nil, valueOrEmpty(patch.Purpose), params.ExpectedVersion)
	if err != nil {
		return model.Application{}, fmt.Errorf("update application: %w", err)
	}
	if result.RowsAffected() == 0 {
		return model.Application{}, r.updateConflict(ctx, params.OrgID, params.ApplicationID, params.ExpectedVersion)
	}
	return r.Get(ctx, params.OrgID, params.ApplicationID)
}

func (r *Repository) updateConflict(ctx context.Context, orgID, applicationID uuid.UUID, expectedVersion int64) error {
	var status string
	var version int64
	err := r.db.QueryRow(ctx, `
		SELECT status, version
		FROM applications
		WHERE id = $1 AND org_id = $2`, applicationID, orgID).Scan(&status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrApplicationNotFound
	}
	if err != nil {
		return fmt.Errorf("check application update conflict: %w", err)
	}
	if status != "DRAFT" {
		return domain.ErrApplicationNotDraft
	}
	if version != expectedVersion {
		return domain.ErrApplicationVersionConflict
	}
	return domain.ErrApplicationVersionConflict
}

func valueOrZero[T int64 | int](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
