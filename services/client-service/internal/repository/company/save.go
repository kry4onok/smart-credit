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
			profit_12m_kopecks, obligations_kopecks, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1)
		ON CONFLICT (org_id) DO NOTHING
		RETURNING inn, name, okved, registered_at, revenue_12m_kopecks,
		          profit_12m_kopecks, obligations_kopecks, version`,
		orgID, c.INN, c.Name, c.OKVED, c.RegisteredAt, c.Revenue12mKopecks,
		c.Profit12mKopecks, c.ObligationsKopecks).Scan(
		&saved.INN, &saved.Name, &saved.OKVED, &saved.RegisteredAt, &saved.Revenue12mKopecks,
		&saved.Profit12mKopecks, &saved.ObligationsKopecks, &saved.Version,
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
		SET inn = $2, name = $3, okved = $4, registered_at = $5,
		    revenue_12m_kopecks = $6, profit_12m_kopecks = $7,
		    obligations_kopecks = $8, version = version + 1, updated_at = now()
		WHERE org_id = $1 AND version = $9
		RETURNING inn, name, okved, registered_at, revenue_12m_kopecks,
		          profit_12m_kopecks, obligations_kopecks, version`,
		orgID, c.INN, c.Name, c.OKVED, c.RegisteredAt, c.Revenue12mKopecks,
		c.Profit12mKopecks, c.ObligationsKopecks, expectedVersion).Scan(
		&saved.INN, &saved.Name, &saved.OKVED, &saved.RegisteredAt, &saved.Revenue12mKopecks,
		&saved.Profit12mKopecks, &saved.ObligationsKopecks, &saved.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Company{}, domain.ErrCompanyVersionConflict
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("update company: %w", err)
	}
	return saved, nil
}
