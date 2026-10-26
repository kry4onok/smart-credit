package company

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) Create(ctx context.Context, orgID uuid.UUID, c model.Company) (model.Company, error) {
	var saved model.Company
	err := r.db.QueryRow(ctx, `
		INSERT INTO companies (
			org_id, inn, name, okved, registered_at, revenue_12m_kopecks,
			profit_12m_kopecks, obligations_kopecks, version, profile_schema_version, legal_form, tax_periods, financial_period, financial_basis, obligations_as_of
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (org_id) DO NOTHING
		RETURNING inn, name, okved, registered_at, revenue_12m_kopecks,
		          profit_12m_kopecks, obligations_kopecks, version, profile_schema_version, legal_form, tax_periods, financial_period, financial_basis, obligations_as_of`,
		orgID, c.INN, c.Name, c.OKVED, c.RegisteredAt, c.Revenue12mKopecks,
		c.Profit12mKopecks, c.ObligationsKopecks, c.ProfileSchemaVersion, c.LegalForm, c.TaxPeriods, c.FinancialPeriod, c.FinancialBasis, c.ObligationsAsOf).Scan(
		&saved.INN, &saved.Name, &saved.OKVED, &saved.RegisteredAt, &saved.Revenue12mKopecks,
		&saved.Profit12mKopecks, &saved.ObligationsKopecks, &saved.Version,
		&saved.ProfileSchemaVersion, &saved.LegalForm, &saved.TaxPeriods, &saved.FinancialPeriod, &saved.FinancialBasis, &saved.ObligationsAsOf,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Company{}, domain.ErrCompanyAlreadyExists
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("create company: %w", err)
	}
	return saved, nil
}

func (r *Repository) Update(ctx context.Context, orgID uuid.UUID, c model.Company, expectedVersion int64) (model.Company, error) {
	var saved model.Company
	err := r.db.QueryRow(ctx, `
		UPDATE companies
		SET name = $3, okved = $4, registered_at = $5,
		    revenue_12m_kopecks = $6, profit_12m_kopecks = $7,
		    obligations_kopecks = $8, profile_schema_version=$10, legal_form=$11, tax_periods=$12, financial_period=$13, financial_basis=$14, obligations_as_of=$15, version = version + 1, updated_at = now()
		WHERE org_id = $1 AND version = $9 AND inn = $2
		RETURNING inn, name, okved, registered_at, revenue_12m_kopecks,
		          profit_12m_kopecks, obligations_kopecks, version, profile_schema_version, legal_form, tax_periods, financial_period, financial_basis, obligations_as_of`,
		orgID, c.INN, c.Name, c.OKVED, c.RegisteredAt, c.Revenue12mKopecks,
		c.Profit12mKopecks, c.ObligationsKopecks, expectedVersion, c.ProfileSchemaVersion, c.LegalForm, c.TaxPeriods, c.FinancialPeriod, c.FinancialBasis, c.ObligationsAsOf).Scan(
		&saved.INN, &saved.Name, &saved.OKVED, &saved.RegisteredAt, &saved.Revenue12mKopecks,
		&saved.Profit12mKopecks, &saved.ObligationsKopecks, &saved.Version,
		&saved.ProfileSchemaVersion, &saved.LegalForm, &saved.TaxPeriods, &saved.FinancialPeriod, &saved.FinancialBasis, &saved.ObligationsAsOf,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		var inn string
		var version int64
		lookupErr := r.db.QueryRow(ctx, "SELECT inn, version FROM companies WHERE org_id=$1", orgID).Scan(&inn, &version)
		if lookupErr == nil && version == expectedVersion && inn != c.INN {
			return model.Company{}, domain.ErrProfileIdentityImmutable
		}
		if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			return model.Company{}, fmt.Errorf("check company identity: %w", lookupErr)
		}
		return model.Company{}, domain.ErrCompanyVersionConflict
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("update company: %w", err)
	}
	return saved, nil
}
