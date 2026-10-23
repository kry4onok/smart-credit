package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

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
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_body
			FROM idempotency_keys
			WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
			params.UserID, idempotencyRoute, params.IdempotencyID).Scan(&storedHash, &responseBody)
		if err != nil {
			return model.Application{}, fmt.Errorf("load idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, requestHash[:]) {
			return model.Application{}, domain.ErrIdempotencyConflict
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
	companySnapshot, err := json.Marshal(companySnapshot(company))
	if err != nil {
		return model.Application{}, fmt.Errorf("encode company snapshot: %w", err)
	}
	productSnapshot, err := json.Marshal(productSnapshot(product))
	if err != nil {
		return model.Application{}, fmt.Errorf("encode product snapshot: %w", err)
	}

	applicationID := uuid.New()
	var application model.Application
	err = tx.QueryRow(ctx, `
		INSERT INTO applications (
			id, org_id, client_user_id, product_id, product_version,
			product_snapshot, company_snapshot, status, stage_label, next_action, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'DRAFT', 'Черновик', 'Заполните заявку', 1)
		RETURNING id, product_id, product_version, status, stage_label,
		          next_action, version, created_at, updated_at`,
		applicationID, params.OrgID, params.UserID, product.ID, product.Version,
		companySnapshot, productSnapshot).Scan(
		&application.ID, &application.ProductID, &application.ProductVersion,
		&application.Status, &application.StageLabel, &application.NextAction,
		&application.Version, &application.CreatedAt, &application.UpdatedAt,
	)
	if err != nil {
		return model.Application{}, fmt.Errorf("create application: %w", err)
	}
	application.Company = company
	application.AllowedActions = []string{"EDIT_DRAFT", "UPLOAD_DOCUMENT", "SUBMIT"}

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
		       profit_12m_kopecks, obligations_kopecks, version
		FROM companies WHERE org_id = $1`, orgID).Scan(
		&company.INN, &company.Name, &company.OKVED, &company.RegisteredAt,
		&company.Revenue12mKopecks, &company.Profit12mKopecks,
		&company.ObligationsKopecks, &company.Version,
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
		       max_term_months, indicative_rate_bps, required_document_types
		FROM products WHERE id = $1`, id).Scan(
		&product.ID, &product.Version, &product.Code, &product.Name, &product.Description,
		&product.PurposeTags, &product.MinAmountKopecks, &product.MaxAmountKopecks,
		&product.MinTermMonths, &product.MaxTermMonths, &product.IndicativeRateBps,
		&product.RequiredDocumentTypes,
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
	INN          string        `json:"inn"`
	Name         string        `json:"name"`
	OKVED        string        `json:"okved"`
	RegisteredAt string        `json:"registered_at"`
	Revenue12m   moneySnapshot `json:"revenue_12m"`
	Profit12m    moneySnapshot `json:"profit_12m"`
	Obligations  moneySnapshot `json:"obligations"`
}

func companySnapshot(value model.Company) companySnapshotJSON {
	return companySnapshotJSON{
		INN: value.INN, Name: value.Name, OKVED: value.OKVED,
		RegisteredAt: value.RegisteredAt.Format("2006-01-02"),
		Revenue12m:   moneySnapshot{AmountKopecks: value.Revenue12mKopecks, Currency: "RUB"},
		Profit12m:    moneySnapshot{AmountKopecks: value.Profit12mKopecks, Currency: "RUB"},
		Obligations:  moneySnapshot{AmountKopecks: value.ObligationsKopecks, Currency: "RUB"},
	}
}

type productSnapshotJSON struct {
	ID                    uuid.UUID     `json:"id"`
	Version               int64         `json:"version"`
	Code                  string        `json:"code"`
	Name                  string        `json:"name"`
	Description           string        `json:"description"`
	PurposeTags           []string      `json:"purpose_tags"`
	MinAmount             moneySnapshot `json:"min_amount"`
	MaxAmount             moneySnapshot `json:"max_amount"`
	MinTermMonths         int           `json:"min_term_months"`
	MaxTermMonths         int           `json:"max_term_months"`
	IndicativeRateBps     *int          `json:"indicative_rate_bps,omitempty"`
	RequiredDocumentTypes []string      `json:"required_document_types"`
}

func productSnapshot(value model.Product) productSnapshotJSON {
	return productSnapshotJSON{
		ID: value.ID, Version: value.Version, Code: value.Code, Name: value.Name,
		Description: value.Description, PurposeTags: value.PurposeTags,
		MinAmount:     moneySnapshot{AmountKopecks: value.MinAmountKopecks, Currency: "RUB"},
		MaxAmount:     moneySnapshot{AmountKopecks: value.MaxAmountKopecks, Currency: "RUB"},
		MinTermMonths: value.MinTermMonths, MaxTermMonths: value.MaxTermMonths,
		IndicativeRateBps: value.IndicativeRateBps, RequiredDocumentTypes: value.RequiredDocumentTypes,
	}
}
