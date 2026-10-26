package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const provideInfoRoutePrefix = "POST /client/applications/"

type infoSnapshot struct {
	requirements       []*commonpb.Requirement
	referenceDate      *time.Time
	status             string
	pendingAction      *string
	version            int64
	clientEventVersion int64
	company            model.Company
	form               model.ApplicationForm
	product            productSnapshotJSON
	documents          []model.ApplicationDocument
	requestID          uuid.UUID
	requiredFields     []string
	requiredDocuments  []string
}

func (r *Repository) ProvideInfo(ctx context.Context, params model.ProvideInfoParams) (model.ActionAccepted, error) {
	route := provideInfoRoutePrefix + params.ApplicationID.String() + "/info"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("begin info response: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reserved int
	err = tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (
			actor_id, route, idempotency_key, request_hash, response_status,
			response_body, application_id, expires_at
		) VALUES ($1, $2, $3, $4, 202, '{}'::jsonb, NULL, now() + interval '24 hours')
		ON CONFLICT (actor_id, route, idempotency_key) DO NOTHING
		RETURNING 1`, params.UserID, route, params.IdempotencyID, params.RequestHash).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var storedHash []byte
		var responseBody []byte
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_body
			FROM idempotency_keys
			WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3
			FOR UPDATE`, params.UserID, route, params.IdempotencyID).Scan(&storedHash, &responseBody)
		if err != nil {
			return model.ActionAccepted{}, fmt.Errorf("load info idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, params.RequestHash) {
			return model.ActionAccepted{}, domain.ErrIdempotencyConflict
		}
		var replay model.ActionAccepted
		if err := json.Unmarshal(responseBody, &replay); err != nil {
			return model.ActionAccepted{}, fmt.Errorf("decode info idempotency response: %w", err)
		}
		return replay, nil
	}
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("reserve info idempotency key: %w", err)
	}

	snapshot, err := r.loadInfoSnapshot(ctx, tx, params.ApplicationID, params.OrgID)
	if err != nil {
		return model.ActionAccepted{}, err
	}
	if snapshot.status != "NEEDS_INFO" || snapshot.pendingAction != nil || snapshot.requestID != params.RequestID {
		if snapshot.product.RequirementsPolicyVersion != nil {
			return model.ActionAccepted{}, domain.ErrStaleInfoRequest
		}
		return model.ActionAccepted{}, domain.ErrApplicationInfoConflict
	}

	company := snapshot.company
	if params.Company != nil {
		if params.Company.INN != snapshot.company.INN {
			return model.ActionAccepted{}, domain.ErrProfileIdentityImmutable
		}
		company = *params.Company
	}
	form := snapshot.form
	if params.Fields != nil {
		if params.Fields.AmountKopecks != nil {
			form.AmountKopecks = *params.Fields.AmountKopecks
		}
		if params.Fields.TermMonths != nil {
			form.TermMonths = *params.Fields.TermMonths
		}
		if params.Fields.Purpose != nil {
			form.Purpose = *params.Fields.Purpose
		}
		if params.Fields.PurposeDescription != nil {
			form.PurposeDescription = *params.Fields.PurposeDescription
		}
	}
	if form.Purpose == "OTHER" && strings.TrimSpace(form.PurposeDescription) == "" {
		return model.ActionAccepted{}, domain.ErrInvalidInfoResponse
	}
	var answered []string
	if snapshot.product.RequirementsPolicyVersion != nil {
		if len(snapshot.requirements) == 0 {
			return model.ActionAccepted{}, domain.ErrStaleInfoRequest
		}
		if form.AmountKopecks < snapshot.product.MinAmount.AmountKopecks || form.AmountKopecks > snapshot.product.MaxAmount.AmountKopecks || form.TermMonths < snapshot.product.MinTermMonths || form.TermMonths > snapshot.product.MaxTermMonths || !contains(snapshot.product.PurposeTags, form.Purpose) {
			return model.ActionAccepted{}, &requirements.RuleError{Code: requirements.RequirementsNotMet, Issues: []requirements.Issue{{Path: "form", Code: requirements.RequirementsNotMet, Message: "Сумма, срок и цель должны соответствовать продукту"}}}
		}
		plan, err := buildPlan(company, snapshot.product, &form, snapshot.documents, snapshot.referenceDate)
		if err != nil {
			return model.ActionAccepted{}, err
		}
		filtered := make([]*commonpb.Requirement, 0, len(snapshot.requirements))
		for _, item := range snapshot.requirements {
			if item.Id == "assessment_input" {
				var raw []byte
				if err := tx.QueryRow(ctx, `SELECT assessment_input FROM applications WHERE id=$1`, params.ApplicationID).Scan(&raw); err != nil {
					return model.ActionAccepted{}, err
				}
				var value struct {
					Financials json.RawMessage `json:"financials"`
				}
				if err := json.Unmarshal(raw, &value); err != nil || len(value.Financials) == 0 || string(value.Financials) == "null" {
					return model.ActionAccepted{}, domain.ErrInvalidApplicationPatch
				}
				continue
			}
			filtered = append(filtered, item)
		}
		snapshot.requirements = filtered
		answered = answeredFields(snapshot.requirements, params)
		if issues := requirements.ValidateResponse(snapshot.requirements, plan, answered); len(issues) > 0 {
			return model.ActionAccepted{}, &requirements.RuleError{Code: issues[0].Code, Issues: issues}
		}
	} else if err := validateInfoResponse(snapshot, params, company, form); err != nil {
		return model.ActionAccepted{}, err
	}

	companyJSON, err := json.Marshal(companySnapshot(company))
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("encode company snapshot for info response: %w", err)
	}

	selectedDocuments, err := selectInfoDocuments(snapshot.documents, snapshot.requiredDocuments, params)
	if snapshot.product.RequirementsPolicyVersion != nil && err == nil {
		selectedDocuments = snapshot.documents
	}
	if err != nil {
		return model.ActionAccepted{}, err
	}

	eventID := uuid.New()
	now := time.Now().UTC()
	clientEventVersion := snapshot.clientEventVersion + 1
	event := &eventspb.Event{
		EventId:          eventID.String(),
		AggregateType:    "APPLICATION",
		AggregateId:      params.ApplicationID.String(),
		AggregateVersion: clientEventVersion,
		OccurredAt:       timestamppb.New(now),
		CorrelationId:    eventID.String(),
		Payload: &eventspb.Event_InfoProvided{InfoProvided: &eventspb.InfoProvided{
			RequestId:              params.RequestID.String(),
			AnsweredRequirementIds: answered,
			Form:                   formProto(form),
			Documents:              documentMetadataProto(selectedDocuments),
			Company:                companyProto(companySnapshot(company)),
		}},
	}
	var assessmentInput []byte
	if err := tx.QueryRow(ctx, `SELECT assessment_input FROM applications WHERE id=$1`, params.ApplicationID).Scan(&assessmentInput); err != nil {
		return model.ActionAccepted{}, err
	}
	event.GetInfoProvided().AssessmentInputJson = assessmentInput
	payload, err := proto.Marshal(event)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("marshal info provided event: %w", err)
	}

	result, err := tx.Exec(ctx, `
		UPDATE applications
		SET company_snapshot = $3,
		    requested_amount_kopecks = $4,
		    requested_term_months = $5,
		    purpose_tag = $6, purpose_description=$10,
		    pending_action = 'INFO_SENT',
		    pending_since = $7,
		    version = $8,
		    client_event_version = $9,
		    updated_at = greatest($7, created_at)
		WHERE id = $1 AND org_id = $2 AND status = 'NEEDS_INFO' AND pending_action IS NULL`,
		params.ApplicationID, params.OrgID, companyJSON, form.AmountKopecks,
		form.TermMonths, form.Purpose, now, snapshot.version+1, clientEventVersion, form.PurposeDescription)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save info response: %w", err)
	}
	if result.RowsAffected() != 1 {
		return model.ActionAccepted{}, domain.ErrApplicationInfoConflict
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO timeline_entries (
			id, application_id, status, action, actor_type, actor_id, message, event_id, occurred_at
		) VALUES ($1, $2, 'NEEDS_INFO', 'INFO_SENT', 'CLIENT', $3,
		          'Дополнения отправлены в банк', $4, $5)`,
		uuid.New(), params.ApplicationID, params.UserID, eventID, now); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("create info response timeline: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
		VALUES ($1, $2, $3, 'InfoProvided', $4)`,
		eventID, params.ApplicationID, clientEventVersion, payload); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save info provided outbox event: %w", err)
	}

	accepted := model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "INFO_SENT"}
	responseBody, err := json.Marshal(accepted)
	if err != nil {
		return model.ActionAccepted{}, fmt.Errorf("encode info response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $4, application_id = $5
		WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
		params.UserID, route, params.IdempotencyID, responseBody, params.ApplicationID); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("save info idempotency response: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ActionAccepted{}, fmt.Errorf("commit info response: %w", err)
	}
	return accepted, nil
}

func (r *Repository) loadInfoSnapshot(ctx context.Context, tx Tx, applicationID, orgID uuid.UUID) (infoSnapshot, error) {
	var snapshot infoSnapshot
	var companyRaw, productRaw []byte
	var amount *int64
	var term *int
	var purpose *string
	var purposeDescription string
	err := tx.QueryRow(ctx, `
		SELECT status, pending_action, version, client_event_version, company_snapshot,
		       product_snapshot, requested_amount_kopecks, requested_term_months, purpose_tag, requirements_reference_date, purpose_description
		FROM applications
		WHERE id = $1 AND org_id = $2
		FOR UPDATE`, applicationID, orgID).Scan(
		&snapshot.status, &snapshot.pendingAction, &snapshot.version, &snapshot.clientEventVersion, &companyRaw,
		&productRaw, &amount, &term, &purpose, &snapshot.referenceDate, &purposeDescription)
	if errors.Is(err, pgx.ErrNoRows) {
		return infoSnapshot{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return infoSnapshot{}, fmt.Errorf("load application for info response: %w", err)
	}
	if err := decodeCompanySnapshot(companyRaw, &snapshot.company); err != nil {
		return infoSnapshot{}, err
	}
	if err := json.Unmarshal(productRaw, &snapshot.product); err != nil {
		return infoSnapshot{}, fmt.Errorf("decode product snapshot for info response: %w", err)
	}
	if amount == nil || term == nil || purpose == nil {
		return infoSnapshot{}, domain.ErrInvalidInfoResponse
	}
	snapshot.form = model.ApplicationForm{AmountKopecks: *amount, TermMonths: *term, Purpose: *purpose, PurposeDescription: purposeDescription}
	err = tx.QueryRow(ctx, `
		SELECT id, required_fields, required_document_types, requirements
		FROM info_requests
		WHERE application_id = $1 AND is_active
		ORDER BY created_at DESC, id DESC
		LIMIT 1
		FOR UPDATE`, applicationID).Scan(&snapshot.requestID, &snapshot.requiredFields, &snapshot.requiredDocuments, &snapshot.requirements)
	if errors.Is(err, pgx.ErrNoRows) {
		return infoSnapshot{}, domain.ErrApplicationInfoConflict
	}
	if err != nil {
		return infoSnapshot{}, fmt.Errorf("load active info request: %w", err)
	}
	rows, err := tx.Query(ctx, `
		SELECT id, document_type, file_name, sha256, is_mock, uploaded_at, requirement_id, coverage_from, coverage_to
		FROM application_documents
		WHERE application_id = $1
		ORDER BY uploaded_at, id`, applicationID)
	if err != nil {
		return infoSnapshot{}, fmt.Errorf("load documents for info response: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var document model.ApplicationDocument
		if err := rows.Scan(&document.ID, &document.DocumentType, &document.FileName,
			&document.Sha256, &document.IsMock, &document.UploadedAt, &document.RequirementID, &document.CoverageFrom, &document.CoverageTo); err != nil {
			return infoSnapshot{}, fmt.Errorf("scan document for info response: %w", err)
		}
		snapshot.documents = append(snapshot.documents, document)
	}
	if err := rows.Err(); err != nil {
		return infoSnapshot{}, fmt.Errorf("iterate documents for info response: %w", err)
	}
	return snapshot, nil
}

func validateInfoResponse(snapshot infoSnapshot, params model.ProvideInfoParams, company model.Company, form model.ApplicationForm) error {
	var issues []string
	if err := domain.ValidateCompany(company); err != nil {
		issues = append(issues, "Проверьте реквизиты и финансовые показатели компании.")
	}
	if form.AmountKopecks < snapshot.product.MinAmount.AmountKopecks ||
		form.AmountKopecks > snapshot.product.MaxAmount.AmountKopecks ||
		form.TermMonths < snapshot.product.MinTermMonths ||
		form.TermMonths > snapshot.product.MaxTermMonths ||
		(!validPurpose(form.Purpose) || !contains(snapshot.product.PurposeTags, form.Purpose)) {
		issues = append(issues, "Проверьте сумму, срок и цель кредита: они должны соответствовать условиям продукта.")
	}
	for _, field := range snapshot.requiredFields {
		switch field {
		case "COMPANY_REVENUE_12M", "COMPANY_PROFIT_12M", "COMPANY_OBLIGATIONS":
			if params.Company == nil {
				issues = append(issues, "Укажите запрошенный показатель: "+infoFieldLabel(field)+".")
			}
		case "APPLICATION_AMOUNT":
			if params.Fields == nil || params.Fields.AmountKopecks == nil {
				issues = append(issues, "Укажите сумму кредита в ответе банку.")
			}
		case "APPLICATION_TERM_MONTHS":
			if params.Fields == nil || params.Fields.TermMonths == nil {
				issues = append(issues, "Укажите срок кредита в ответе банку.")
			}
		case "APPLICATION_PURPOSE":
			if params.Fields == nil || params.Fields.Purpose == nil {
				issues = append(issues, "Укажите цель кредита в ответе банку.")
			}
		default:
			issues = append(issues, "Запрос банка содержит неподдерживаемое поле. Обновите карточку заявки.")
		}
	}
	return domain.InvalidFields(domain.ErrInvalidInfoResponse, issues...)
}

func infoFieldLabel(value string) string {
	switch value {
	case "COMPANY_REVENUE_12M":
		return "выручка за 12 месяцев"
	case "COMPANY_PROFIT_12M":
		return "прибыль за 12 месяцев"
	case "COMPANY_OBLIGATIONS":
		return "текущие кредиты и займы"
	default:
		return "показатель компании"
	}
}

func selectInfoDocuments(documents []model.ApplicationDocument, required []string, params model.ProvideInfoParams) ([]model.ApplicationDocument, error) {
	if !params.DocumentsSpecified {
		if !hasRequiredDocuments(documents, required) {
			return nil, missingInfoDocuments(documents, required)
		}
		return documents, nil
	}
	byID := make(map[uuid.UUID]model.ApplicationDocument, len(documents))
	for _, document := range documents {
		byID[document.ID] = document
	}
	selected := make([]model.ApplicationDocument, 0, len(params.DocumentIDs))
	seen := make(map[uuid.UUID]struct{}, len(params.DocumentIDs))
	for _, id := range params.DocumentIDs {
		if id == uuid.Nil {
			return nil, domain.ErrInvalidInfoResponse
		}
		if _, ok := seen[id]; ok {
			return nil, domain.ErrInvalidInfoResponse
		}
		document, ok := byID[id]
		if !ok {
			return nil, domain.ErrInvalidInfoResponse
		}
		seen[id] = struct{}{}
		selected = append(selected, document)
	}
	if !hasRequiredDocuments(selected, required) {
		return nil, missingInfoDocuments(selected, required)
	}
	return selected, nil
}

func missingInfoDocuments(documents []model.ApplicationDocument, required []string) error {
	var issues []string
	for _, kind := range required {
		if !hasRequiredDocuments(documents, []string{kind}) {
			issues = append(issues, "Приложите запрошенный документ: "+documentLabel(kind)+".")
		}
	}
	return domain.InvalidFields(domain.ErrInvalidInfoResponse, issues...)
}

func hasRequiredDocuments(documents []model.ApplicationDocument, required []string) bool {
	for _, wanted := range required {
		found := false
		for _, document := range documents {
			if document.DocumentType == wanted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func answeredFields(request []*commonpb.Requirement, params model.ProvideInfoParams) []string {
	ids := []string{}
	for _, r := range request {
		if r.GetKind() != commonpb.RequirementKind_REQUIREMENT_KIND_FIELD {
			continue
		}
		present := false
		if c := params.Company; c != nil {
			switch r.GetFieldPath() {
			case "company.legal_form":
				present = c.LegalForm != nil
			case "company.tax_periods":
				present = c.TaxPeriods != nil
			case "company.revenue_12m":
				present = c.Revenue12mKopecks != nil
			case "company.profit_12m":
				present = c.Profit12mKopecks != nil
			case "company.obligations":
				present = c.ObligationsKopecks != nil
			case "company.financial_period":
				present = c.FinancialPeriod != nil
			case "company.financial_basis":
				present = c.FinancialBasis != nil
			case "company.obligations_as_of":
				present = c.ObligationsAsOf != nil
			}
		}
		if f := params.Fields; f != nil {
			switch r.GetFieldPath() {
			case "form.amount":
				present = f.AmountKopecks != nil
			case "form.term_months":
				present = f.TermMonths != nil
			case "form.purpose":
				present = f.Purpose != nil
			case "form.purpose_description":
				present = f.PurposeDescription != nil
			}
		}
		if present {
			ids = append(ids, r.GetId())
		}
	}
	return ids
}
