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

func (r *Repository) Fetch(ctx context.Context, orgID uuid.UUID) (model.Company, error) {
	var c model.Company
	err := r.db.QueryRow(ctx, `
		SELECT inn, name, okved, registered_at, revenue_12m_kopecks,
		       profit_12m_kopecks, obligations_kopecks, version, profile_schema_version, legal_form, tax_periods, financial_period, financial_basis, obligations_as_of
		FROM companies WHERE org_id = $1`, orgID).Scan(
		&c.INN, &c.Name, &c.OKVED, &c.RegisteredAt, &c.Revenue12mKopecks,
		&c.Profit12mKopecks, &c.ObligationsKopecks, &c.Version,
		&c.ProfileSchemaVersion, &c.LegalForm, &c.TaxPeriods, &c.FinancialPeriod, &c.FinancialBasis, &c.ObligationsAsOf,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Company{}, domain.ErrCompanyNotFound
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("fetch company: %w", err)
	}
	return c, nil
}
