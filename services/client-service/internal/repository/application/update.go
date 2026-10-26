package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) Update(ctx context.Context, params model.UpdateApplicationParams) (model.Application, error) {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.Application{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var version int64
	var companyRaw, productRaw []byte
	var ref *time.Time
	err = tx.QueryRow(ctx, `SELECT status,version,company_snapshot,product_snapshot,requirements_reference_date FROM applications WHERE id=$1 AND org_id=$2 FOR UPDATE`, params.ApplicationID, params.OrgID).Scan(&status, &version, &companyRaw, &productRaw, &ref)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Application{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return model.Application{}, err
	}
	if status != "DRAFT" {
		return model.Application{}, domain.ErrApplicationNotDraft
	}
	if version != params.ExpectedVersion {
		return model.Application{}, domain.ErrApplicationVersionConflict
	}
	patch := params.Patch
	if patch.Company != nil {
		var previous model.Company
		if err := decodeCompanySnapshot(companyRaw, &previous); err != nil {
			return model.Application{}, err
		}
		if previous.INN != patch.Company.INN {
			return model.Application{}, domain.ErrProfileIdentityImmutable
		}
		validationDate := time.Now().UTC()
		if ref != nil {
			validationDate = *ref
		}
		if err := domain.ValidateCompanyAt(*patch.Company, validationDate); err != nil {
			return model.Application{}, err
		}
		var product productSnapshotJSON
		if err := json.Unmarshal(productRaw, &product); err != nil {
			return model.Application{}, err
		}
		if product.RequirementsPolicyVersion != nil {
			if _, err := buildPlan(*patch.Company, product, nil, nil, ref); err != nil {
				return model.Application{}, err
			}
		}
		companyRaw, err = json.Marshal(companySnapshot(*patch.Company))
		if err != nil {
			return model.Application{}, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE applications SET requested_amount_kopecks=CASE WHEN $3::boolean THEN $4::bigint ELSE requested_amount_kopecks END,
 requested_term_months=CASE WHEN $5::boolean THEN $6::integer ELSE requested_term_months END,
 purpose_tag=CASE WHEN $7::boolean THEN $8::text ELSE purpose_tag END,
 purpose_description=CASE WHEN $10::boolean THEN $11::text ELSE purpose_description END,
 company_snapshot=$9,version=version+1,updated_at=now() WHERE id=$1 AND org_id=$2`, params.ApplicationID, params.OrgID, patch.AmountKopecks != nil, valueOrZero(patch.AmountKopecks), patch.TermMonths != nil, valueOrZero(patch.TermMonths), patch.Purpose != nil, valueOrEmpty(patch.Purpose), companyRaw, patch.PurposeDescription != nil, valueOrEmpty(patch.PurposeDescription))
	if err != nil {
		return model.Application{}, fmt.Errorf("update draft: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Application{}, err
	}
	return r.Get(ctx, params.OrgID, params.ApplicationID)
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
