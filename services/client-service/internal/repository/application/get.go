package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) Get(ctx context.Context, orgID, applicationID uuid.UUID) (model.Application, error) {
	var application model.Application
	var companySnapshot []byte
	var amount *int64
	var termMonths *int
	var purpose *string
	var hasSignature bool

	err := r.db.QueryRow(ctx, `
		SELECT id, product_id, product_version, company_snapshot,
		       requested_amount_kopecks, requested_term_months, purpose_tag,
		       status, stage_label, next_action, pending_action, public_reason,
		       version, created_at, updated_at,
		       EXISTS (SELECT 1 FROM timeline_entries WHERE application_id = applications.id AND action = 'SIGN')
		FROM applications
		WHERE id = $1 AND org_id = $2`, applicationID, orgID).Scan(
		&application.ID, &application.ProductID, &application.ProductVersion, &companySnapshot,
		&amount, &termMonths, &purpose, &application.Status, &application.StageLabel,
		&application.NextAction, &application.PendingAction, &application.PublicReason,
		&application.Version, &application.CreatedAt, &application.UpdatedAt, &hasSignature,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Application{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return model.Application{}, fmt.Errorf("fetch application: %w", err)
	}

	if err := decodeCompanySnapshot(companySnapshot, &application.Company); err != nil {
		return model.Application{}, fmt.Errorf("decode company snapshot: %w", err)
	}
	if amount != nil && termMonths != nil && purpose != nil {
		application.Form = &model.ApplicationForm{
			AmountKopecks: *amount,
			TermMonths:    *termMonths,
			Purpose:       *purpose,
		}
	}

	application.Documents, err = r.documents(ctx, applicationID)
	if err != nil {
		return model.Application{}, err
	}
	application.InfoRequest, err = r.infoRequest(ctx, applicationID)
	if err != nil {
		return model.Application{}, err
	}
	application.Offer, err = r.offer(ctx, applicationID, application.Status)
	if err != nil {
		return model.Application{}, err
	}
	application.AllowedActions = allowedActions(application.Status, application.PendingAction, hasSignature)
	return application, nil
}

func decodeCompanySnapshot(raw []byte, company *model.Company) error {
	var snapshot companySnapshotJSON
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return err
	}
	registeredAt, err := time.Parse("2006-01-02", snapshot.RegisteredAt)
	if err != nil {
		return err
	}
	*company = model.Company{
		INN:                snapshot.INN,
		Name:               snapshot.Name,
		OKVED:              snapshot.OKVED,
		RegisteredAt:       registeredAt,
		Revenue12mKopecks:  snapshot.Revenue12m.AmountKopecks,
		Profit12mKopecks:   snapshot.Profit12m.AmountKopecks,
		ObligationsKopecks: snapshot.Obligations.AmountKopecks,
	}
	return nil
}

func (r *Repository) documents(ctx context.Context, applicationID uuid.UUID) ([]model.ApplicationDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, document_type, file_name, sha256, is_mock, uploaded_at
		FROM application_documents
		WHERE application_id = $1
		ORDER BY uploaded_at, id`, applicationID)
	if err != nil {
		return nil, fmt.Errorf("fetch application documents: %w", err)
	}
	defer rows.Close()

	documents := make([]model.ApplicationDocument, 0)
	for rows.Next() {
		var document model.ApplicationDocument
		if err := rows.Scan(&document.ID, &document.DocumentType, &document.FileName,
			&document.Sha256, &document.IsMock, &document.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan application document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application documents: %w", err)
	}
	return documents, nil
}

func (r *Repository) infoRequest(ctx context.Context, applicationID uuid.UUID) (*model.ApplicationInfoRequest, error) {
	var request model.ApplicationInfoRequest
	err := r.db.QueryRow(ctx, `
		SELECT id, message, required_fields, required_document_types, created_at
		FROM info_requests
		WHERE application_id = $1 AND is_active
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, applicationID).Scan(
		&request.ID, &request.Message, &request.RequiredFields,
		&request.RequiredDocumentTypes, &request.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch active info request: %w", err)
	}
	return &request, nil
}

func (r *Repository) offer(ctx context.Context, applicationID uuid.UUID, status string) (*model.ApplicationOffer, error) {
	var offer model.ApplicationOffer
	// Acceptance deactivates the offer, but signing still needs its ID and terms.
	// In FORMALIZING select the latest offer, as SignContract does.
	err := r.db.QueryRow(ctx, `
		SELECT id, kind, product_id, product_version, product_name,
		       amount_kopecks, term_months, rate_bps, explanation, expires_at
		FROM offers
		WHERE application_id = $1 AND (is_active OR $2 = 'FORMALIZING')
		ORDER BY received_at DESC, id DESC
		LIMIT 1`, applicationID, status).Scan(
		&offer.ID, &offer.Kind, &offer.ProductID, &offer.ProductVersion,
		&offer.ProductName, &offer.AmountKopecks, &offer.TermMonths,
		&offer.RateBps, &offer.Explanation, &offer.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch application offer: %w", err)
	}
	return &offer, nil
}

func allowedActions(status string, pendingAction *string, hasSignature bool) []string {
	if pendingAction != nil {
		return []string{}
	}
	actions := make([]string, 0)
	switch status {
	case "DRAFT":
		actions = append(actions, "EDIT_DRAFT", "UPLOAD_DOCUMENT", "SUBMIT")
	case "NEEDS_INFO":
		actions = append(actions, "PROVIDE_INFO", "UPLOAD_DOCUMENT")
	case "OFFER_AVAILABLE":
		actions = append(actions, "ACCEPT_OFFER", "DECLINE_OFFER")
	case "FORMALIZING":
		if !hasSignature {
			actions = append(actions, "SIGN")
		}
	}
	if domain.CanCancelApplication(status, pendingAction, hasSignature) {
		actions = append(actions, "CANCEL")
	}
	return actions
}
