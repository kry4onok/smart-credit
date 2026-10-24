package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const submitIdempotencyRoutePrefix = "POST /client/applications/"

type submitSnapshot struct {
	application        model.Application
	company            companySnapshotJSON
	product            productSnapshotJSON
	documents          []model.ApplicationDocument
	clientEventVersion int64
}

func (r *Repository) Submit(ctx context.Context, params model.SubmitApplicationParams) (model.Application, error) {
	requestHash := sha256.Sum256([]byte(params.ApplicationID.String()))
	route := submitIdempotencyRoutePrefix + params.ApplicationID.String() + "/submit"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.Application{}, fmt.Errorf("begin application submission: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reserved int
	err = tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (
			actor_id, route, idempotency_key, request_hash, response_status,
			response_body, application_id, expires_at
		) VALUES ($1, $2, $3, $4, 200, '{}'::jsonb, NULL, now() + interval '24 hours')
		ON CONFLICT (actor_id, route, idempotency_key) DO NOTHING
		RETURNING 1`, params.UserID, route, params.IdempotencyID, requestHash[:]).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var storedHash []byte
		var responseBody []byte
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_body
			FROM idempotency_keys
			WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3
			FOR UPDATE`, params.UserID, route, params.IdempotencyID).Scan(&storedHash, &responseBody)
		if err != nil {
			return model.Application{}, fmt.Errorf("load submission idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, requestHash[:]) {
			return model.Application{}, domain.ErrIdempotencyConflict
		}
		var replay model.Application
		if err := json.Unmarshal(responseBody, &replay); err != nil {
			return model.Application{}, fmt.Errorf("decode submission idempotency response: %w", err)
		}
		if replay.ID == uuid.Nil {
			return model.Application{}, errors.New("decode submission idempotency response: empty application id")
		}
		return replay, nil
	}
	if err != nil {
		return model.Application{}, fmt.Errorf("reserve submission idempotency key: %w", err)
	}

	snapshot, err := r.loadSubmitSnapshot(ctx, tx, params)
	if err != nil {
		return model.Application{}, err
	}
	if snapshot.application.Status != "DRAFT" {
		return model.Application{}, domain.ErrApplicationNotDraft
	}
	if err := validateSubmitSnapshot(snapshot); err != nil {
		return model.Application{}, err
	}

	eventID := uuid.New()
	updatedAt := time.Now().UTC()
	newVersion := snapshot.application.Version + 1
	newClientEventVersion := snapshot.clientEventVersion + 1
	application := snapshot.application
	application.Status = "SUBMITTED"
	application.StageLabel = "Отправлена"
	application.NextAction = "Ожидайте решения банка"
	application.AllowedActions = []string{}
	application.Version = newVersion
	application.UpdatedAt = updatedAt
	application.Documents = snapshot.documents
	form := *snapshot.application.Form
	application.Form = &form

	if err := tx.QueryRow(ctx, `
		UPDATE applications
		SET status = 'SUBMITTED', stage_label = 'Отправлена',
		    next_action = 'Ожидайте решения банка', version = $3,
		    client_event_version = $4, submitted_at = now(), updated_at = now()
		WHERE id = $1 AND org_id = $2 AND status = 'DRAFT'
		RETURNING version, client_event_version, updated_at`,
		params.ApplicationID, params.OrgID, newVersion, newClientEventVersion).Scan(
		&application.Version, &newClientEventVersion, &application.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Application{}, domain.ErrApplicationNotDraft
		}
		return model.Application{}, fmt.Errorf("submit application: %w", err)
	}

	event := &eventspb.Event{
		EventId:          eventID.String(),
		AggregateType:    "APPLICATION",
		AggregateId:      params.ApplicationID.String(),
		AggregateVersion: newClientEventVersion,
		OccurredAt:       timestamppb.New(application.UpdatedAt),
		CorrelationId:    eventID.String(),
		Payload: &eventspb.Event_ApplicationSubmitted{ApplicationSubmitted: &eventspb.ApplicationSubmitted{
			OrgId:          params.OrgID.String(),
			ClientUserId:   params.UserID.String(),
			ProductId:      snapshot.application.ProductID.String(),
			ProductVersion: snapshot.application.ProductVersion,
			Company:        companyProto(snapshot.company),
			Form:           formProto(*snapshot.application.Form),
			Documents:      documentMetadataProto(snapshot.documents),
		}},
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return model.Application{}, fmt.Errorf("marshal application submitted event: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO timeline_entries (
			id, application_id, status, action, actor_type, actor_id, message, event_id, occurred_at
		) VALUES ($1, $2, 'SUBMITTED', 'SUBMITTED', 'CLIENT', $3,
		          'Заявка отправлена в банк', $4, $5)`,
		uuid.New(), params.ApplicationID, params.UserID, eventID, application.UpdatedAt); err != nil {
		return model.Application{}, fmt.Errorf("create submission timeline: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (event_id, aggregate_id, aggregate_version, event_type, payload)
		VALUES ($1, $2, $3, 'ApplicationSubmitted', $4)`,
		eventID, params.ApplicationID, newClientEventVersion, payload); err != nil {
		return model.Application{}, fmt.Errorf("save application submitted outbox event: %w", err)
	}

	responseBody, err := json.Marshal(application)
	if err != nil {
		return model.Application{}, fmt.Errorf("encode submission idempotency response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $4, application_id = $5
		WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
		params.UserID, route, params.IdempotencyID, responseBody, params.ApplicationID); err != nil {
		return model.Application{}, fmt.Errorf("save submission idempotency response: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Application{}, fmt.Errorf("commit application submission: %w", err)
	}
	return application, nil
}

func (r *Repository) loadSubmitSnapshot(ctx context.Context, tx Tx, params model.SubmitApplicationParams) (submitSnapshot, error) {
	var result submitSnapshot
	var companyRaw, productRaw []byte
	var amount *int64
	var term *int
	var purpose *string
	var clientUserID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id, client_user_id, product_id, product_version,
		       company_snapshot, product_snapshot, requested_amount_kopecks,
		       requested_term_months, purpose_tag, status, version,
		       client_event_version, created_at, updated_at
		FROM applications
		WHERE id = $1 AND org_id = $2
		FOR UPDATE`, params.ApplicationID, params.OrgID).Scan(
		&result.application.ID, &clientUserID, &result.application.ProductID,
		&result.application.ProductVersion, &companyRaw, &productRaw, &amount,
		&term, &purpose, &result.application.Status, &result.application.Version,
		&result.clientEventVersion, &result.application.CreatedAt, &result.application.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return submitSnapshot{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return submitSnapshot{}, fmt.Errorf("load application for submission: %w", err)
	}
	result.application.Company = model.Company{}
	if err := json.Unmarshal(companyRaw, &result.company); err != nil {
		return submitSnapshot{}, fmt.Errorf("decode company snapshot for submission: %w", err)
	}
	registeredAt, err := time.Parse("2006-01-02", result.company.RegisteredAt)
	if err != nil {
		return submitSnapshot{}, fmt.Errorf("parse company snapshot for submission: %w", err)
	}
	result.application.Company = model.Company{
		INN: result.company.INN, Name: result.company.Name, OKVED: result.company.OKVED,
		RegisteredAt: registeredAt, Revenue12mKopecks: result.company.Revenue12m.AmountKopecks,
		Profit12mKopecks: result.company.Profit12m.AmountKopecks, ObligationsKopecks: result.company.Obligations.AmountKopecks,
	}
	if err := json.Unmarshal(productRaw, &result.product); err != nil {
		return submitSnapshot{}, fmt.Errorf("decode product snapshot for submission: %w", err)
	}
	if amount != nil && term != nil && purpose != nil {
		result.application.Form = &model.ApplicationForm{AmountKopecks: *amount, TermMonths: *term, Purpose: *purpose}
	}
	rows, err := tx.Query(ctx, `
		SELECT id, document_type, file_name, sha256, is_mock, uploaded_at
		FROM application_documents
		WHERE application_id = $1
		ORDER BY uploaded_at, id`, params.ApplicationID)
	if err != nil {
		return submitSnapshot{}, fmt.Errorf("load application documents for submission: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var document model.ApplicationDocument
		if err := rows.Scan(&document.ID, &document.DocumentType, &document.FileName,
			&document.Sha256, &document.IsMock, &document.UploadedAt); err != nil {
			return submitSnapshot{}, fmt.Errorf("scan application document for submission: %w", err)
		}
		result.documents = append(result.documents, document)
	}
	if err := rows.Err(); err != nil {
		return submitSnapshot{}, fmt.Errorf("iterate application documents for submission: %w", err)
	}
	return result, nil
}

func validateSubmitSnapshot(snapshot submitSnapshot) error {
	if snapshot.application.Form == nil || snapshot.company.RegisteredAt == "" ||
		snapshot.application.Form.AmountKopecks <= 0 || snapshot.application.Form.TermMonths <= 0 ||
		!validPurpose(snapshot.application.Form.Purpose) {
		return domain.ErrInvalidApplication
	}
	if snapshot.application.Form.AmountKopecks < snapshot.product.MinAmount.AmountKopecks ||
		snapshot.application.Form.AmountKopecks > snapshot.product.MaxAmount.AmountKopecks ||
		snapshot.application.Form.TermMonths < snapshot.product.MinTermMonths ||
		snapshot.application.Form.TermMonths > snapshot.product.MaxTermMonths {
		return domain.ErrInvalidApplication
	}
	for _, required := range snapshot.product.RequiredDocumentTypes {
		found := false
		for _, document := range snapshot.documents {
			if document.DocumentType == required {
				found = true
				break
			}
		}
		if !found {
			return domain.ErrInvalidApplication
		}
	}
	return nil
}

func validPurpose(value string) bool {
	switch value {
	case "WORKING_CAPITAL", "EQUIPMENT", "EXPANSION":
		return true
	default:
		return false
	}
}

func companyProto(value companySnapshotJSON) *commonpb.CompanySnapshot {
	return &commonpb.CompanySnapshot{
		Inn: value.INN, Name: value.Name, Okved: value.OKVED, RegisteredAt: value.RegisteredAt,
		Revenue_12M: moneyProto(value.Revenue12m), Profit_12M: moneyProto(value.Profit12m),
		Obligations: moneyProto(value.Obligations),
	}
}

func moneyProto(value moneySnapshot) *commonpb.Money {
	return &commonpb.Money{AmountKopecks: value.AmountKopecks, Currency: value.Currency}
}

func formProto(value model.ApplicationForm) *commonpb.ApplicationForm {
	return &commonpb.ApplicationForm{
		Amount:     moneyProto(moneySnapshot{AmountKopecks: value.AmountKopecks, Currency: "RUB"}),
		TermMonths: int32(value.TermMonths), Purpose: commonpb.PurposeTag(commonpb.PurposeTag_value["PURPOSE_TAG_"+value.Purpose]),
	}
}

func documentMetadataProto(values []model.ApplicationDocument) []*commonpb.DocumentMetadata {
	result := make([]*commonpb.DocumentMetadata, 0, len(values))
	for _, value := range values {
		result = append(result, &commonpb.DocumentMetadata{
			DocumentId: value.ID.String(), DocumentType: commonpb.DocumentType(commonpb.DocumentType_value["DOCUMENT_TYPE_"+value.DocumentType]),
			FileName: value.FileName, Sha256: value.Sha256, UploadedAt: timestamppb.New(value.UploadedAt), IsMock: value.IsMock,
		})
	}
	return result
}
