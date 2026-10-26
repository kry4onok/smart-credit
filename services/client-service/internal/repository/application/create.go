package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const idempotencyRoute = "/client/applications"

func (r *Repository) Create(ctx context.Context, params model.CreateApplicationParams) (model.Application, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.Application{}, fmt.Errorf("begin application creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	requestHash := sha256.Sum256([]byte(params.ProductID.String()))
	var actorID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (
			actor_id, route, idempotency_key, request_hash, response_status,
			response_body, application_id, expires_at
		) VALUES ($1, $2, $3, $4, 201, '{}'::jsonb, NULL, now() + interval '24 hours')
		ON CONFLICT (actor_id, route, idempotency_key) DO NOTHING
		RETURNING actor_id`,
		params.UserID, idempotencyRoute, params.IdempotencyID, requestHash[:]).Scan(&actorID)
	if errors.Is(err, pgx.ErrNoRows) {
		var storedHash []byte
		var responseBody []byte
		var responseStatus int
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_body, response_status
			FROM idempotency_keys
			WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
			params.UserID, idempotencyRoute, params.IdempotencyID).Scan(&storedHash, &responseBody, &responseStatus)
		if err != nil {
			return model.Application{}, fmt.Errorf("load idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, requestHash[:]) {
			return model.Application{}, domain.ErrIdempotencyConflict
		}
		if responseStatus == 410 {
			return model.Application{}, domain.ErrApplicationNotFound
		}
		var replay model.Application
		if err := json.Unmarshal(responseBody, &replay); err != nil {
			return model.Application{}, fmt.Errorf("decode idempotent response: %w", err)
		}
		return replay, nil
	}
	if err != nil {
		return model.Application{}, fmt.Errorf("reserve idempotency key: %w", err)
	}
	company, err := fetchCompany(ctx, tx, params.OrgID)
	if err != nil {
		return model.Application{}, err
	}
	product, err := fetchProduct(ctx, tx, params.ProductID)
	if err != nil {
		return model.Application{}, err
	}
	if product.RequirementsPolicyVersion != nil && (!requirements.SupportedPolicy(*product.RequirementsPolicyVersion) || company.ProfileSchemaVersion == nil || *company.ProfileSchemaVersion != 2) {
		return model.Application{}, &requirements.RuleError{Code: requirements.InvalidProfile, Issues: []requirements.Issue{{Path: "company.profile_schema_version", Code: requirements.InvalidProfile, Message: "Продукт требует профиль v2 и поддержанную политику"}}}
	}
	companySnapshot, err := json.Marshal(companySnapshot(company))
	if err != nil {
		return model.Application{}, fmt.Errorf("encode company snapshot: %w", err)
	}
	productSnapshotRaw, err := json.Marshal(productSnapshot(product))
	if err != nil {
		return model.Application{}, fmt.Errorf("encode product snapshot: %w", err)
	}

	applicationID := uuid.New()
	var application model.Application
	err = tx.QueryRow(ctx, `
		INSERT INTO applications (
			id, org_id, client_user_id, product_id, product_version,
			product_snapshot, company_snapshot, status, stage_label, next_action, version, requirements_reference_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'DRAFT', 'Черновик', 'Заполните заявку', 1, CASE WHEN $8::boolean THEN (now() AT TIME ZONE 'UTC')::date ELSE NULL END)
		RETURNING id, product_id, product_version, status, stage_label,
		          next_action, version, created_at, updated_at, requirements_reference_date`,
		applicationID, params.OrgID, params.UserID, product.ID, product.Version,
		productSnapshotRaw, companySnapshot, product.RequirementsPolicyVersion != nil).Scan(
		&application.ID, &application.ProductID, &application.ProductVersion,
		&application.Status, &application.StageLabel, &application.NextAction,
		&application.Version, &application.CreatedAt, &application.UpdatedAt, &application.RequirementsReferenceDate,
	)
	if err != nil {
		return model.Application{}, fmt.Errorf("create application: %w", err)
	}
	application.Company = company
	if product.RequirementsPolicyVersion != nil {
		application.RequirementsPlan, err = buildPlan(company, productSnapshot(product), nil, nil, application.RequirementsReferenceDate)
		if err != nil {
			return model.Application{}, err
		}
	}
	application.AllowedActions = []string{"EDIT_DRAFT", "DELETE_DRAFT", "UPLOAD_DOCUMENT", "SUBMIT"}

	if _, err := tx.Exec(ctx, `
		INSERT INTO timeline_entries (
			id, application_id, status, action, actor_type, actor_id, message
		) VALUES ($1, $2, 'DRAFT', 'CREATED', 'CLIENT', $3, 'Черновик создан')`,
		uuid.New(), application.ID, params.UserID); err != nil {
		return model.Application{}, fmt.Errorf("create application timeline: %w", err)
	}

	responseBody, err := json.Marshal(application)
	if err != nil {
		return model.Application{}, fmt.Errorf("encode idempotent response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $4, application_id = $5
		WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
		params.UserID, idempotencyRoute, params.IdempotencyID, responseBody, application.ID); err != nil {
		return model.Application{}, fmt.Errorf("save idempotent response: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Application{}, fmt.Errorf("commit application creation: %w", err)
	}
	return application, nil
}

func fetchCompany(ctx context.Context, tx Tx, orgID uuid.UUID) (model.Company, error) {
	var company model.Company
	err := tx.QueryRow(ctx, `
		SELECT inn, name, okved, registered_at, revenue_12m_kopecks,
		       profit_12m_kopecks, obligations_kopecks, version, profile_schema_version, legal_form, tax_periods, financial_period, financial_basis, obligations_as_of
		FROM companies WHERE org_id = $1`, orgID).Scan(
		&company.INN, &company.Name, &company.OKVED, &company.RegisteredAt,
		&company.Revenue12mKopecks, &company.Profit12mKopecks,
		&company.ObligationsKopecks, &company.Version,
		&company.ProfileSchemaVersion, &company.LegalForm, &company.TaxPeriods, &company.FinancialPeriod, &company.FinancialBasis, &company.ObligationsAsOf,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Company{}, domain.ErrCompanyNotFound
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("fetch company: %w", err)
	}
	return company, nil
}

func fetchProduct(ctx context.Context, tx Tx, id uuid.UUID) (model.Product, error) {
	var product model.Product
	err := tx.QueryRow(ctx, `
		SELECT id, version, code, name, description, purpose_tags,
		       min_amount_kopecks, max_amount_kopecks, min_term_months,
		       max_term_months, indicative_rate_bps, required_document_types, requirements_policy_version
		FROM products WHERE id = $1`, id).Scan(
		&product.ID, &product.Version, &product.Code, &product.Name, &product.Description,
		&product.PurposeTags, &product.MinAmountKopecks, &product.MaxAmountKopecks,
		&product.MinTermMonths, &product.MaxTermMonths, &product.IndicativeRateBps,
		&product.RequiredDocumentTypes, &product.RequirementsPolicyVersion,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Product{}, domain.ErrProductNotFound
	}
	if err != nil {
		return model.Product{}, fmt.Errorf("fetch product: %w", err)
	}
	return product, nil
}

type moneySnapshot struct {
	AmountKopecks int64  `json:"amount_kopecks"`
	Currency      string `json:"currency"`
}

type companySnapshotJSON struct {
	model.ProfileDetails
	INN          string         `json:"inn"`
	Name         string         `json:"name"`
	OKVED        string         `json:"okved"`
	RegisteredAt string         `json:"registered_at"`
	Revenue12m   *moneySnapshot `json:"revenue_12m,omitempty"`
	Profit12m    *moneySnapshot `json:"profit_12m,omitempty"`
	Obligations  *moneySnapshot `json:"obligations,omitempty"`
}

func companySnapshot(value model.Company) companySnapshotJSON {
	return companySnapshotJSON{ProfileDetails: value.ProfileDetails, INN: value.INN, Name: value.Name, OKVED: value.OKVED, RegisteredAt: value.RegisteredAt.Format(time.DateOnly), Revenue12m: optionalMoney(value.Revenue12mKopecks), Profit12m: optionalMoney(value.Profit12mKopecks), Obligations: optionalMoney(value.ObligationsKopecks)}
}
func optionalMoney(v *int64) *moneySnapshot {
	if v == nil {
		return nil
	}
	return &moneySnapshot{AmountKopecks: *v, Currency: "RUB"}
}
func snapshotCompany(v companySnapshotJSON) model.Company {
	registered, _ := time.Parse(time.DateOnly, v.RegisteredAt)
	c := model.Company{ProfileDetails: v.ProfileDetails, INN: v.INN, Name: v.Name, OKVED: v.OKVED, RegisteredAt: registered}
	if v.Revenue12m != nil {
		c.Revenue12mKopecks = profile.Ptr(v.Revenue12m.AmountKopecks)
	}
	if v.Profit12m != nil {
		c.Profit12mKopecks = profile.Ptr(v.Profit12m.AmountKopecks)
	}
	if v.Obligations != nil {
		c.ObligationsKopecks = profile.Ptr(v.Obligations.AmountKopecks)
	}
	return c
}

type productSnapshotJSON struct {
	RequirementsPolicyVersion *string       `json:"requirements_policy_version,omitempty"`
	ID                        uuid.UUID     `json:"id"`
	Version                   int64         `json:"version"`
	Code                      string        `json:"code"`
	Name                      string        `json:"name"`
	Description               string        `json:"description"`
	PurposeTags               []string      `json:"purpose_tags"`
	MinAmount                 moneySnapshot `json:"min_amount"`
	MaxAmount                 moneySnapshot `json:"max_amount"`
	MinTermMonths             int           `json:"min_term_months"`
	MaxTermMonths             int           `json:"max_term_months"`
	IndicativeRateBps         *int          `json:"indicative_rate_bps,omitempty"`
	RequiredDocumentTypes     []string      `json:"required_document_types"`
}

func productSnapshot(value model.Product) productSnapshotJSON {
	return productSnapshotJSON{
		RequirementsPolicyVersion: value.RequirementsPolicyVersion,
		ID:                        value.ID, Version: value.Version, Code: value.Code, Name: value.Name,
		Description: value.Description, PurposeTags: value.PurposeTags,
		MinAmount:     moneySnapshot{AmountKopecks: value.MinAmountKopecks, Currency: "RUB"},
		MaxAmount:     moneySnapshot{AmountKopecks: value.MaxAmountKopecks, Currency: "RUB"},
		MinTermMonths: value.MinTermMonths, MaxTermMonths: value.MaxTermMonths,
		IndicativeRateBps: value.IndicativeRateBps, RequiredDocumentTypes: value.RequiredDocumentTypes,
	}
}
