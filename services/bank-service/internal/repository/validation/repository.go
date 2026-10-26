package validationrepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

const infoRequestMessage = "Для продолжения рассмотрения дополните данные заявки."

type Repository struct {
	db transaction.Queryer
}

func New(db transaction.Queryer) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ClaimNext(ctx context.Context) (validation.Claim, bool, error) {
	var claim validation.Claim
	err := transaction.QueryerFromContext(ctx, r.db).QueryRow(ctx, claimNextQuery).Scan(
		&claim.CaseID, &claim.Version, &claim.ProductID, &claim.ProductVersion,
		&claim.Snapshot.CompanyJSON, &claim.Snapshot.DocumentsJSON,
		&claim.Snapshot.ReferenceDate,
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
		&product.PolicyVersion, &product.PurposeTags, &product.RequiredDocumentTypes,
		&product.MinAmountKopecks, &product.MaxAmountKopecks,
		&product.MinTermMonths, &product.MaxTermMonths,
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
		if outcome.PublicReason != "" {
			publicReason = outcome.PublicReason
		}
		triggeredRules = []string{outcome.Reason}
	case model.CaseStatusManualReview:
		recommendation = "MANUAL_REVIEW"
		if outcome.PublicReason != "" {
			publicReason = outcome.PublicReason
		}
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
		if outcome.Plan != nil {
			data, encodeErr := encodeRequirements(outcome.RequestedRequirements)
			if encodeErr != nil {
				return fmt.Errorf("encode v2 request: %w", encodeErr)
			}
			_, err = q.Exec(ctx, insertV2InfoRequestQuery, requestID, claim.CaseID, infoRequestMessage, string(data))
			if err != nil {
				return fmt.Errorf("create v2 validation request: %w", err)
			}
			payload, err = publicstate.NeedsInfo(eventID, claim.CaseID, requestID, version, now, infoRequestMessage, nil, nil)
			if err != nil {
				return fmt.Errorf("encode v2 needs-info event: %w", err)
			}
			payload, err = publicstate.WithRequirements(payload, outcome.Plan, outcome.RequestedRequirements)
			if err != nil {
				return fmt.Errorf("add requirements to public event: %w", err)
			}
		} else {
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
		}
	} else {
		if outcome.Status == model.CaseStatusManualReview && outcome.PublicReason != "" {
			payload, err = publicstate.ManualReview(eventID, claim.CaseID, version, now, outcome.PublicReason)
			if err != nil {
				return fmt.Errorf("encode manual public event: %w", err)
			}
		} else {
			payload = publicstate.UnderReview(eventID, claim.CaseID, claim.CaseID, version, now)
		}
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
	_, err = publicstate.Publish(ctx, q, eventID, claim.CaseID.String(), version, payload)
	if err != nil {
		return fmt.Errorf("write validation outbox: %w", err)
	}
	return nil
}

func encodeRequirements(items []*commonpb.Requirement) ([]byte, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("empty v2 request")
	}
	raw := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		if item == nil {
			return nil, fmt.Errorf("nil requirement")
		}
		data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(item)
		if err != nil {
			return nil, err
		}
		raw = append(raw, data)
	}
	return json.Marshal(raw)
}
