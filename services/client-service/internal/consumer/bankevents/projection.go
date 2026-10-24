package bankevents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrEventConflict = errors.New("bank event ID or aggregate version conflicts with a different event")

type Tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type DB interface {
	Begin(context.Context) (pgx.Tx, error)
}

type Projection struct{ db DB }

func NewProjection(db DB) *Projection { return &Projection{db: db} }

func (p *Projection) Execute(ctx context.Context, event *eventspb.Event) (bool, error) {
	if err := validateEnvelope(event); err != nil {
		return false, err
	}
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin bank event projection: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	applied, err := reserveEvent(ctx, tx, event)
	if err != nil || !applied {
		return false, err
	}
	switch {
	case event.GetApplicationPublicStatePublished() != nil:
		err = applyApplicationState(ctx, tx, event)
	case event.GetProductCatalogPublished() != nil:
		err = applyCatalog(ctx, tx, event)
	default:
		err = fmt.Errorf("unsupported bank event payload")
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit bank event projection: %w", err)
	}
	return true, nil
}

func validateEnvelope(event *eventspb.Event) error {
	if event == nil || event.GetEventId() == "" || event.GetAggregateId() == "" ||
		event.GetAggregateVersion() <= 0 || event.GetOccurredAt() == nil ||
		event.GetOccurredAt().CheckValid() != nil || strings.TrimSpace(event.GetCorrelationId()) == "" {
		return errors.New("invalid bank event envelope")
	}
	if _, err := uuid.Parse(event.GetEventId()); err != nil {
		return fmt.Errorf("invalid bank event_id: %w", err)
	}
	if event.GetApplicationPublicStatePublished() != nil {
		if event.GetAggregateType() != "APPLICATION" {
			return errors.New("application public state has invalid aggregate type")
		}
		if _, err := uuid.Parse(event.GetAggregateId()); err != nil {
			return fmt.Errorf("invalid application aggregate_id: %w", err)
		}
		return nil
	}
	if event.GetProductCatalogPublished() != nil {
		if event.GetAggregateType() != "CATALOG" || event.GetAggregateId() != "catalog" {
			return errors.New("product catalog has invalid aggregate")
		}
		return nil
	}
	return errors.New("unsupported bank event payload")
}

func reserveEvent(ctx context.Context, tx Tx, event *eventspb.Event) (bool, error) {
	var reserved int
	err := tx.QueryRow(ctx, `
		INSERT INTO processed_events (event_id, topic, aggregate_id, aggregate_version)
		VALUES ($1, 'bank.events', $2, $3)
		ON CONFLICT DO NOTHING
		RETURNING 1`, event.GetEventId(), event.GetAggregateId(), event.GetAggregateVersion()).Scan(&reserved)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("reserve bank event: %w", err)
	}
	var storedID string
	err = tx.QueryRow(ctx, `
		SELECT event_id
		FROM processed_events
		WHERE topic = 'bank.events' AND aggregate_id = $1 AND aggregate_version = $2`,
		event.GetAggregateId(), event.GetAggregateVersion()).Scan(&storedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrEventConflict
	}
	if err != nil {
		return false, fmt.Errorf("read duplicate bank event: %w", err)
	}
	if storedID != event.GetEventId() {
		return false, ErrEventConflict
	}
	return false, nil
}

func applyApplicationState(ctx context.Context, tx Tx, event *eventspb.Event) error {
	state := event.GetApplicationPublicStatePublished()
	status, err := clientStatus(state.GetStatus())
	if err != nil {
		return err
	}
	applicationID, _ := uuid.Parse(event.GetAggregateId())
	var publicReason any
	if state.GetPublicReason() != "" {
		publicReason = state.GetPublicReason()
	}
	result, err := tx.Exec(ctx, `
		UPDATE applications
		SET status = $2, stage_label = $3, next_action = $4, public_reason = $5,
		    pending_action = NULL, pending_since = NULL,
		    bank_event_version = $6, version = version + 1, updated_at = now()
		WHERE id = $1 AND bank_event_version < $6`,
		applicationID, status, state.GetStageLabel(), state.GetNextAction(), publicReason, event.GetAggregateVersion())
	if err != nil {
		return fmt.Errorf("update application public state: %w", err)
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM applications WHERE id = $1)`, applicationID).Scan(&exists); err != nil {
			return fmt.Errorf("check projected application: %w", err)
		}
		if !exists {
			return fmt.Errorf("application %s not found for bank event", applicationID)
		}
		return nil
	}

	if _, err := tx.Exec(ctx, `UPDATE info_requests SET is_active = false, responded_at = now() WHERE application_id = $1 AND is_active`, applicationID); err != nil {
		return fmt.Errorf("deactivate info request: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE offers SET is_active = false WHERE application_id = $1 AND is_active`, applicationID); err != nil {
		return fmt.Errorf("deactivate offer: %w", err)
	}
	if request := state.GetActiveInfoRequest(); request != nil {
		if err := insertInfoRequest(ctx, tx, applicationID, request); err != nil {
			return err
		}
	}
	if offer := state.GetActiveOffer(); offer != nil {
		if err := insertOffer(ctx, tx, applicationID, offer); err != nil {
			return err
		}
	}
	message := state.GetStageLabel()
	if message == "" {
		message = state.GetNextAction()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO timeline_entries (id, application_id, status, action, actor_type, message, event_id, occurred_at)
		VALUES ($1, $2, $3, 'BANK_STATE', 'SYSTEM', $4, $5, $6)`,
		uuid.New(), applicationID, status, message, event.GetEventId(), event.GetOccurredAt().AsTime()); err != nil {
		return fmt.Errorf("create bank state timeline: %w", err)
	}
	return nil
}

func applyCatalog(ctx context.Context, tx Tx, event *eventspb.Event) error {
	var latestVersion int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(aggregate_version), 0)
		FROM processed_events
		WHERE topic = 'bank.events' AND aggregate_id = 'catalog' AND event_id <> $1`, event.GetEventId()).Scan(&latestVersion); err != nil {
		return fmt.Errorf("read current catalog version: %w", err)
	}
	if latestVersion > event.GetAggregateVersion() {
		return nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM products`); err != nil {
		return fmt.Errorf("replace client product catalog: %w", err)
	}
	for _, product := range event.GetProductCatalogPublished().GetProducts() {
		if err := insertProduct(ctx, tx, product); err != nil {
			return err
		}
	}
	return nil
}

func insertProduct(ctx context.Context, tx Tx, product *commonpb.Product) error {
	if product == nil || product.GetProductId() == "" || product.GetVersion() <= 0 || product.GetCode() == "" || product.GetName() == "" ||
		product.GetMinAmount() == nil || product.GetMaxAmount() == nil || product.GetMinTermMonths() <= 0 || product.GetMaxTermMonths() < product.GetMinTermMonths() {
		return errors.New("invalid product catalog item")
	}
	id, err := uuid.Parse(product.GetProductId())
	if err != nil {
		return fmt.Errorf("invalid catalog product id: %w", err)
	}
	if err := validateMoney(product.GetMinAmount(), false); err != nil {
		return err
	}
	if err := validateMoney(product.GetMaxAmount(), false); err != nil {
		return err
	}
	tags := make([]string, 0, len(product.GetPurposeTags()))
	for _, tag := range product.GetPurposeTags() {
		switch tag {
		case commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL, commonpb.PurposeTag_PURPOSE_TAG_EQUIPMENT, commonpb.PurposeTag_PURPOSE_TAG_EXPANSION:
			tags = append(tags, strings.TrimPrefix(tag.String(), "PURPOSE_TAG_"))
		default:
			return errors.New("invalid catalog purpose tag")
		}
	}
	documents := make([]string, 0, len(product.GetRequiredDocumentTypes()))
	for _, documentType := range product.GetRequiredDocumentTypes() {
		switch documentType {
		case commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT, commonpb.DocumentType_DOCUMENT_TYPE_BUSINESS_PLAN:
			documents = append(documents, strings.TrimPrefix(documentType.String(), "DOCUMENT_TYPE_"))
		default:
			return errors.New("invalid catalog document type")
		}
	}
	var indicativeRate any
	if product.GetIndicativeRateBps() != 0 {
		indicativeRate = product.GetIndicativeRateBps()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO products (
			id, version, code, name, description, purpose_tags,
			min_amount_kopecks, max_amount_kopecks, min_term_months,
			max_term_months, indicative_rate_bps, required_document_types
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		id, product.GetVersion(), product.GetCode(), product.GetName(), product.GetDescription(), tags,
		product.GetMinAmount().GetAmountKopecks(), product.GetMaxAmount().GetAmountKopecks(),
		product.GetMinTermMonths(), product.GetMaxTermMonths(), indicativeRate, documents); err != nil {
		return fmt.Errorf("save catalog product: %w", err)
	}
	return nil
}

func insertInfoRequest(ctx context.Context, tx Tx, applicationID uuid.UUID, request *commonpb.InfoRequest) error {
	requestID, err := uuid.Parse(request.GetRequestId())
	if err != nil || requestID == uuid.Nil || request.GetCreatedAt() == nil || request.GetCreatedAt().CheckValid() != nil || strings.TrimSpace(request.GetMessage()) == "" {
		return errors.New("invalid bank info request")
	}
	fields := make([]string, 0, len(request.GetRequiredFields()))
	for _, field := range request.GetRequiredFields() {
		if field == commonpb.InfoField_INFO_FIELD_UNSPECIFIED {
			return errors.New("invalid bank info field")
		}
		fields = append(fields, strings.TrimPrefix(field.String(), "INFO_FIELD_"))
	}
	documents := make([]string, 0, len(request.GetRequiredDocumentTypes()))
	for _, documentType := range request.GetRequiredDocumentTypes() {
		switch documentType {
		case commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT, commonpb.DocumentType_DOCUMENT_TYPE_BUSINESS_PLAN:
			documents = append(documents, strings.TrimPrefix(documentType.String(), "DOCUMENT_TYPE_"))
		default:
			return errors.New("invalid bank info document type")
		}
	}
	if len(fields) == 0 && len(documents) == 0 {
		return errors.New("bank info request is empty")
	}
	result, err := tx.Exec(ctx, `
		INSERT INTO info_requests (id, application_id, message, required_fields, required_document_types, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET message = EXCLUDED.message, required_fields = EXCLUDED.required_fields,
		    required_document_types = EXCLUDED.required_document_types,
		    created_at = EXCLUDED.created_at, is_active = true, responded_at = NULL
		WHERE info_requests.application_id = EXCLUDED.application_id`,
		requestID, applicationID, request.GetMessage(), fields, documents, request.GetCreatedAt().AsTime())
	if err != nil {
		return fmt.Errorf("save bank info request: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("bank info request belongs to another application")
	}
	return nil
}

func insertOffer(ctx context.Context, tx Tx, applicationID uuid.UUID, offer *commonpb.Offer) error {
	offerID, err := uuid.Parse(offer.GetOfferId())
	if err != nil || offerID == uuid.Nil || offer.GetProductId() == "" || offer.GetProductVersion() <= 0 || offer.GetAmount() == nil || offer.GetTermMonths() <= 0 || offer.GetRateBps() < 0 || offer.GetExpiresAt() == nil || offer.GetExpiresAt().CheckValid() != nil {
		return errors.New("invalid bank offer")
	}
	productID, err := uuid.Parse(offer.GetProductId())
	if err != nil || productID == uuid.Nil {
		return errors.New("invalid bank offer product id")
	}
	if err := validateMoney(offer.GetAmount(), false); err != nil {
		return err
	}
	var kind string
	switch offer.GetKind() {
	case commonpb.OfferKind_OFFER_KIND_OFFER:
		kind = "OFFER"
	case commonpb.OfferKind_OFFER_KIND_COUNTER_OFFER:
		kind = "COUNTER_OFFER"
	default:
		return errors.New("invalid bank offer kind")
	}
	if strings.TrimSpace(offer.GetProductName()) == "" {
		return errors.New("bank offer product name is empty")
	}
	result, err := tx.Exec(ctx, `
		INSERT INTO offers (
			id, application_id, kind, product_id, product_version, product_name,
			amount_kopecks, term_months, rate_bps, explanation, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE
		SET kind = EXCLUDED.kind, product_id = EXCLUDED.product_id,
		    product_version = EXCLUDED.product_version, product_name = EXCLUDED.product_name,
		    amount_kopecks = EXCLUDED.amount_kopecks, term_months = EXCLUDED.term_months,
		    rate_bps = EXCLUDED.rate_bps, explanation = EXCLUDED.explanation,
		    expires_at = EXCLUDED.expires_at, is_active = true
		WHERE offers.application_id = EXCLUDED.application_id`,
		offerID, applicationID, kind, productID, offer.GetProductVersion(), offer.GetProductName(),
		offer.GetAmount().GetAmountKopecks(), offer.GetTermMonths(), offer.GetRateBps(), offer.GetExplanation(), offer.GetExpiresAt().AsTime())
	if err != nil {
		return fmt.Errorf("save bank offer: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("bank offer belongs to another application")
	}
	return nil
}

func validateMoney(value *commonpb.Money, allowNegative bool) error {
	if value == nil || value.GetCurrency() != "RUB" || (!allowNegative && value.GetAmountKopecks() < 0) {
		return errors.New("invalid money in bank event")
	}
	return nil
}

func clientStatus(value eventspb.ClientStatus) (string, error) {
	switch value {
	case eventspb.ClientStatus_CLIENT_STATUS_SUBMITTED:
		return "SUBMITTED", nil
	case eventspb.ClientStatus_CLIENT_STATUS_UNDER_REVIEW:
		return "UNDER_REVIEW", nil
	case eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO:
		return "NEEDS_INFO", nil
	case eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE:
		return "OFFER_AVAILABLE", nil
	case eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING:
		return "FORMALIZING", nil
	case eventspb.ClientStatus_CLIENT_STATUS_DISBURSED:
		return "DISBURSED", nil
	case eventspb.ClientStatus_CLIENT_STATUS_REJECTED:
		return "REJECTED", nil
	case eventspb.ClientStatus_CLIENT_STATUS_CLOSED:
		return "CLOSED", nil
	default:
		return "", errors.New("invalid client status in bank event")
	}
}
