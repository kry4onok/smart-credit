package receiverepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrEventConflict = errors.New("client event ID or aggregate version conflicts with a different event")
var ErrInfoProvidedConflict = errors.New("InfoProvided does not match an open request on a NEEDS_INFO case")
var ErrOfferResponseConflict = errors.New("offer response does not match a current, unexpired offer")
var ErrCancellationConflict = errors.New("cancellation requires a current case before signing")
var ErrSigningConflict = errors.New("contract signing requires the accepted offer and next client event version")

type Repository struct {
	db transaction.Queryer
}

func New(db transaction.Queryer) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Apply(ctx context.Context, input receive.Input) (bool, error) {
	if len(input.AssessmentInputJSON) == 0 {
		input.AssessmentInputJSON = []byte(`{"schema_version":1,"preferences":{}}`)
	}
	q := transaction.QueryerFromContext(ctx, r.db)
	var reserved int
	err := q.QueryRow(ctx, reserveEventQuery,
		input.EventID, input.ApplicationID, input.AggregateVersion).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var aggregateID uuid.UUID
		var version int64
		err = q.QueryRow(ctx, selectDuplicateEventQuery, input.EventID).
			Scan(&aggregateID, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrEventConflict
		}
		if err != nil {
			return false, fmt.Errorf("read duplicate event: %w", err)
		}
		if aggregateID != input.ApplicationID || version != input.AggregateVersion {
			return false, ErrEventConflict
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reserve event: %w", err)
	}
	if input.InfoProvided {
		applied, err := r.applyInfoProvided(ctx, q, input)
		if err == nil && applied {
			_, err = q.Exec(ctx, `UPDATE cases SET assessment_input=$2::jsonb WHERE id=$1`, input.ApplicationID, string(input.AssessmentInputJSON))
		}
		return applied, err
	}
	if input.Cancelled {
		return r.applyCancellation(ctx, q, input)
	}
	if input.OfferAction == "ContractSigned" {
		return r.applyContractSigned(ctx, q, input)
	}
	if input.OfferAction != "" {
		return r.applyOfferResponse(ctx, q, input)
	}

	status := "VALIDATION"
	var recommendation *string
	var scheduleValidation bool
	var triggeredRules []string
	var productName string
	var policyVersion *string
	markManual := func(code string) {
		status, scheduleValidation = "MANUAL_REVIEW", false
		manualReview := "MANUAL_REVIEW"
		recommendation = &manualReview
		triggeredRules = []string{code}
	}
	err = q.QueryRow(ctx, selectProductQuery,
		input.ProductID, input.ProductVersion).Scan(&productName, &policyVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		productName = "Неизвестный продукт"
		markManual("UNKNOWN_PRODUCT")
	} else if err != nil {
		return false, fmt.Errorf("load product: %w", err)
	} else {
		scheduleValidation = true
		triggeredRules = []string{}
		if policyVersion != nil && requirements.SupportedPolicy(*policyVersion) {
			if input.RequirementsReferenceDate == nil || input.ProfileSchemaVersion != 2 {
				markManual("PROFILE_V2_INVALID")
			} else if _, _, conflict, mergeErr := mergeDocumentSnapshots([]byte(`[]`), input.DocumentsJSON); mergeErr != nil {
				return false, fmt.Errorf("validate submitted documents: %w", mergeErr)
			} else if conflict {
				markManual("DOCUMENT_METADATA_CONFLICT")
			}
		} else if policyVersion != nil {
			markManual("UNKNOWN_REQUIREMENTS_POLICY")
		} else if input.RequirementsReferenceDate != nil || input.ProfileSchemaVersion == 2 {
			markManual("PROFILE_V2_POLICY_MISMATCH")
		}
	}

	_, err = q.Exec(ctx, insertCaseQuery,
		input.ApplicationID, input.OrgID, input.ClientUserID,
		input.ProductID, input.ProductVersion, productName,
		input.CompanyINN, input.CompanyName, string(input.CompanyJSON), string(input.FormJSON),
		string(input.DocumentsJSON), input.AmountKopecks, input.TermMonths,
		input.PurposeTag, status, recommendation, triggeredRules, scheduleValidation,
		input.AggregateVersion, input.RequirementsReferenceDate, string(input.AssessmentInputJSON))
	if err != nil {
		return false, fmt.Errorf("create case: %w", err)
	}

	_, err = q.Exec(ctx, insertAuditQuery,
		uuid.New(), input.ApplicationID, input.ClientUserID,
		reason(status, triggeredRules), input.EventID.String())
	if err != nil {
		return false, fmt.Errorf("audit submitted application: %w", err)
	}
	return true, nil
}

func (r *Repository) applyCancellation(ctx context.Context, q transaction.Queryer, input receive.Input) (bool, error) {
	var status string
	var caseVersion, clientEventVersion int64
	var clientUserID uuid.UUID
	err := q.QueryRow(ctx, selectCaseForOfferQuery, input.ApplicationID).
		Scan(&status, &caseVersion, &clientEventVersion, &clientUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrCancellationConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock case for cancellation: %w", err)
	}
	if !cancellableStatus(status) || input.AggregateVersion != clientEventVersion+1 {
		return false, ErrCancellationConflict
	}
	var newVersion int64
	err = q.QueryRow(ctx, updateCaseForCancellationQuery, input.ApplicationID, status,
		input.AggregateVersion, caseVersion).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrCancellationConflict
	}
	if err != nil {
		return false, fmt.Errorf("cancel case: %w", err)
	}
	if _, err := q.Exec(ctx, cancelOpenInfoRequestQuery, input.ApplicationID); err != nil {
		return false, fmt.Errorf("cancel open info request: %w", err)
	}
	if _, err := q.Exec(ctx, declineAvailableOfferQuery, input.ApplicationID); err != nil {
		return false, fmt.Errorf("close available offer: %w", err)
	}
	var now time.Time
	if err := q.QueryRow(ctx, selectCancellationTimeQuery).Scan(&now); err != nil {
		return false, fmt.Errorf("read cancellation time: %w", err)
	}
	if _, err := q.Exec(ctx, insertCancellationAuditQuery, uuid.New(), input.ApplicationID,
		newVersion, clientUserID, status, input.EventID, input.CancelReason, now); err != nil {
		return false, fmt.Errorf("audit cancellation: %w", err)
	}
	eventID := uuid.New()
	payload, err := publicstate.Cancelled(eventID, input.ApplicationID, newVersion, now)
	if err != nil {
		return false, fmt.Errorf("encode cancellation: %w", err)
	}
	if _, err := publicstate.Publish(ctx, q, eventID, input.ApplicationID.String(), newVersion, payload); err != nil {
		return false, fmt.Errorf("write cancellation outbox: %w", err)
	}
	return true, nil
}

func cancellableStatus(status string) bool {
	switch status {
	case "SUBMITTED", "VALIDATION", "NEEDS_INFO", "ENRICHMENT", "SCORING",
		"DECISION", "MANUAL_REVIEW", "OFFER", "COUNTER_OFFER", "OFFER_ACCEPTED":
		return true
	default:
		return false
	}
}

func (r *Repository) applyOfferResponse(ctx context.Context, q transaction.Queryer, input receive.Input) (bool, error) {
	if input.OfferAction != "OfferAccepted" && input.OfferAction != "OfferDeclined" {
		return false, ErrOfferResponseConflict
	}
	var status string
	var caseVersion, clientEventVersion int64
	var clientUserID uuid.UUID
	err := q.QueryRow(ctx, selectCaseForOfferQuery, input.ApplicationID).
		Scan(&status, &caseVersion, &clientEventVersion, &clientUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrOfferResponseConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock case for offer response: %w", err)
	}
	if (status != "OFFER" && status != "COUNTER_OFFER") || input.AggregateVersion != clientEventVersion+1 {
		return false, ErrOfferResponseConflict
	}
	var offerID uuid.UUID
	var expiresAt, now time.Time
	err = q.QueryRow(ctx, selectCurrentOfferQuery, input.ApplicationID).Scan(&offerID, &expiresAt, &now)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrOfferResponseConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock current offer: %w", err)
	}
	if offerID != input.OfferID {
		return false, ErrOfferResponseConflict
	}

	newStatus, offerStatus := "OFFER_ACCEPTED", "ACCEPTED"
	auditAction, publicAction := input.OfferAction, input.OfferAction
	if input.OfferAction == "OfferDeclined" {
		newStatus, offerStatus = "CANCELLED", "DECLINED"
	}
	updateQuery := updateOfferResponseQuery
	if !expiresAt.After(now) {
		newStatus, offerStatus = "EXPIRED", "EXPIRED"
		auditAction, publicAction = "OFFER_RESPONSE_EXPIRED", "OfferExpired"
		updateQuery = expireOfferResponseQuery
	}
	result, err := q.Exec(ctx, updateQuery, input.OfferID, input.ApplicationID, offerStatus, now)
	if err != nil {
		return false, fmt.Errorf("update offer response: %w", err)
	}
	if result.RowsAffected() != 1 {
		return false, ErrOfferResponseConflict
	}
	var newVersion int64
	err = q.QueryRow(ctx, updateCaseForOfferQuery, input.ApplicationID, status, newStatus,
		input.AggregateVersion, caseVersion, now).Scan(&newVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrOfferResponseConflict
	}
	if err != nil {
		return false, fmt.Errorf("update case for offer response: %w", err)
	}
	_, err = q.Exec(ctx, insertOfferResponseAuditQuery, uuid.New(), input.ApplicationID, newVersion,
		auditAction, clientUserID, status, newStatus, input.EventID, input.OfferID, now)
	if err != nil {
		return false, fmt.Errorf("audit offer response: %w", err)
	}
	eventID := uuid.New()
	payload, err := publicstate.OfferResponse(eventID, input.ApplicationID, newVersion, now, publicAction)
	if err != nil {
		return false, fmt.Errorf("encode offer response: %w", err)
	}
	_, err = publicstate.Publish(ctx, q, eventID, input.ApplicationID.String(), newVersion, payload)
	if err != nil {
		return false, fmt.Errorf("write offer response outbox: %w", err)
	}
	return true, nil
}

func (r *Repository) applyInfoProvided(ctx context.Context, q transaction.Queryer, input receive.Input) (bool, error) {
	var status string
	var clientEventVersion int64
	var companyINN string
	var storedDocuments []byte
	var referenceDate *time.Time
	var productID uuid.UUID
	var productVersion int64
	err := q.QueryRow(ctx, selectCaseForInfoQuery, input.ApplicationID).Scan(
		&status, &clientEventVersion, &companyINN, &storedDocuments,
		&referenceDate, &productID, &productVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock case for InfoProvided: %w", err)
	}
	if status != "NEEDS_INFO" || input.AggregateVersion <= clientEventVersion {
		return false, ErrInfoProvidedConflict
	}
	var requestID uuid.UUID
	var requestJSON []byte
	err = q.QueryRow(ctx, selectOpenInfoRequestQuery, input.ApplicationID).Scan(&requestID, &requestJSON)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && requestID != input.RequestID) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, fmt.Errorf("lock open info request: %w", err)
	}
	if referenceDate != nil && string(requestJSON) != "[]" {
		return r.applyV2InfoProvided(ctx, q, input, companyINN, storedDocuments,
			*referenceDate, productID, productVersion, requestJSON)
	}
	if input.InvalidDocumentMetadata {
		return false, receive.ErrInvalidEvent
	}
	result, err := q.Exec(ctx, fulfillInfoRequestQuery, input.RequestID, input.ApplicationID)
	if err != nil {
		return false, fmt.Errorf("fulfill info request: %w", err)
	}
	if result.RowsAffected() != 1 {
		return false, ErrInfoProvidedConflict
	}
	var version int64
	err = q.QueryRow(ctx, updateCaseFromInfoQuery,
		input.ApplicationID, input.CompanyINN, input.CompanyName,
		string(input.CompanyJSON), string(input.FormJSON), string(input.DocumentsJSON),
		input.AmountKopecks, input.TermMonths, input.PurposeTag, input.AggregateVersion).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, fmt.Errorf("update case from InfoProvided: %w", err)
	}
	_, err = q.Exec(ctx, insertInfoProvidedAuditQuery, uuid.New(), input.ApplicationID, version,
		input.EventID.String(), input.RequestID.String())
	if err != nil {
		return false, fmt.Errorf("audit InfoProvided: %w", err)
	}
	return true, nil
}

func reason(status string, triggeredRules []string) string {
	if status == "MANUAL_REVIEW" && len(triggeredRules) > 0 {
		return triggeredRules[0]
	}
	return ""
}
