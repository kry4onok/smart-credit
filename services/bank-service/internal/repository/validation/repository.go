package validationrepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const infoRequestMessage = "Для продолжения рассмотрения дополните данные заявки."

type Repository struct {
	db transaction.Queryer
}

func New(db transaction.Queryer) *Repository {
	return &Repository{db: db}
}

// ClaimNext is called within the use case transaction. The row lock lasts
// through validation and all writes, which do not involve external services.
func (r *Repository) ClaimNext(ctx context.Context) (validation.Claim, bool, error) {
	var claim validation.Claim
	err := transaction.QueryerFromContext(ctx, r.db).QueryRow(ctx, claimNextQuery).Scan(
		&claim.CaseID, &claim.Version, &claim.ProductID, &claim.ProductVersion,
		&claim.Snapshot.CompanyJSON, &claim.Snapshot.DocumentsJSON,
		&claim.Snapshot.AmountKopecks, &claim.Snapshot.TermMonths, &claim.Snapshot.PurposeTag,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return validation.Claim{}, false, nil
	}
	if err != nil {
		return validation.Claim{}, false, fmt.Errorf("claim validation case: %w", err)
	}
	return claim, true, nil
}

func (r *Repository) LoadProduct(ctx context.Context, claim validation.Claim) (validation.Product, error) {
	var product validation.Product
	err := transaction.QueryerFromContext(ctx, r.db).QueryRow(ctx, loadProductQuery,
		claim.ProductID, claim.ProductVersion).Scan(
		&product.PurposeTags, &product.RequiredDocumentTypes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return validation.Product{}, nil
	}
	if err != nil {
		return validation.Product{}, fmt.Errorf("load validation product: %w", err)
	}
	product.Found = true
	return product, nil
}

func (r *Repository) Complete(ctx context.Context, claim validation.Claim, outcome validation.Outcome) error {
	q := transaction.QueryerFromContext(ctx, r.db)
	var recommendation any
	var publicReason any
	triggeredRules := []string{}
	switch outcome.Status {
	case model.CaseStatusNeedsInfo:
		recommendation = "REQUEST_INFO"
		publicReason = infoRequestMessage
		triggeredRules = []string{outcome.Reason}
	case model.CaseStatusManualReview:
		recommendation = "MANUAL_REVIEW"
		triggeredRules = []string{outcome.Reason}
	case model.CaseStatusEnrichment:
	default:
		return fmt.Errorf("unsupported validation outcome %q", outcome.Status)
	}
	var version int64
	err := q.QueryRow(ctx, finishCaseQuery, claim.CaseID, claim.Version,
		string(outcome.Status), recommendation, triggeredRules, publicReason).Scan(&version)
	if err != nil {
		return fmt.Errorf("finish validation case: %w", err)
	}
	now := time.Now().UTC()
	eventID := uuid.New()
	var payload []byte
	if outcome.Status == model.CaseStatusNeedsInfo {
		requestID := uuid.New()
		_, err = q.Exec(ctx, insertInfoRequestQuery, requestID, claim.CaseID, infoRequestMessage,
			outcome.RequiredFields, outcome.RequiredDocumentTypes)
		if err != nil {
			return fmt.Errorf("create validation info request: %w", err)
		}
		payload, err = publicstate.NeedsInfo(eventID, claim.CaseID, requestID, version, now,
			infoRequestMessage, outcome.RequiredFields, outcome.RequiredDocumentTypes)
		if err != nil {
			return fmt.Errorf("encode needs-info event: %w", err)
		}
	} else {
		payload = publicstate.UnderReview(eventID, claim.CaseID, claim.CaseID, version, now)
	}
	details, err := json.Marshal(map[string]any{
		"to_status": outcome.Status, "required_fields": outcome.RequiredFields,
		"required_document_types": outcome.RequiredDocumentTypes,
	})
	if err != nil {
		return fmt.Errorf("encode validation audit: %w", err)
	}
	_, err = q.Exec(ctx, insertAuditQuery, uuid.New(), claim.CaseID, version,
		outcome.Reason, string(details))
	if err != nil {
		return fmt.Errorf("audit validation: %w", err)
	}
	_, err = q.Exec(ctx, insertOutboxQuery, eventID, claim.CaseID.String(), version, payload)
	if err != nil {
		return fmt.Errorf("write validation outbox: %w", err)
	}
	return nil
}
