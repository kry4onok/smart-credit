package decisionrepository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	assessmentrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/assessment"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/decision"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const offerExplanation = "Заявка соответствует правилам выбранного продукта."

type Repository struct {
	db           transaction.Queryer
	transactions *transaction.Manager
}

func New(db transaction.Queryer, transactions *transaction.Manager) *Repository {
	return &Repository{db: db, transactions: transactions}
}

func (r *Repository) RunOnce(ctx context.Context) (bool, error) {
	return assessmentrepository.New(r.db, r.transactions).RunOnce(ctx)
}

func complete(ctx context.Context, q transaction.Queryer, caseID, productID uuid.UUID, version int64,
	scoreID *int64, in decision.Input, result decision.Result,
) error {
	decisionID := uuid.New()
	var storedProductID any
	var ruleVersion any
	var storedProductVersion any
	if in.Product.Found {
		storedProductID = productID
		ruleVersion = in.Product.Version
		storedProductVersion = in.Product.Version
	}
	var publicReason any
	if result.PublicReason != "" {
		publicReason = result.PublicReason
	}
	if _, err := q.Exec(ctx, insertDecisionQuery, decisionID, caseID, result.Outcome, publicReason,
		storedProductID, storedProductVersion, scoreID, ruleVersion, result.Rules); err != nil {
		return fmt.Errorf("insert automatic decision: %w", err)
	}
	now := in.Now
	eventID := uuid.New()
	var payload []byte
	var err error
	switch result.Status {
	case model.CaseStatusOffer:
		offerID := uuid.New()
		expires := now.Add(time.Duration(in.Product.OfferTTLHours) * time.Hour)
		if _, err = q.Exec(ctx, insertOfferQuery, offerID, caseID, decisionID, productID,
			in.Product.Version, in.Product.Name, in.Snapshot.AmountKopecks, in.Snapshot.TermMonths,
			result.RateBP, offerExplanation, expires); err != nil {
			return fmt.Errorf("insert automatic offer: %w", err)
		}
		offer := &commonpb.Offer{OfferId: offerID.String(), Kind: commonpb.OfferKind_OFFER_KIND_OFFER,
			ProductId: productID.String(), ProductVersion: in.Product.Version, ProductName: in.Product.Name,
			Amount:     &commonpb.Money{AmountKopecks: in.Snapshot.AmountKopecks, Currency: "RUB"},
			TermMonths: in.Snapshot.TermMonths, RateBps: int32(result.RateBP),
			ExpiresAt: timestamppb.New(expires), Explanation: offerExplanation}
		payload, err = publicstate.Decision(eventID, caseID, version+1, now, "APPROVE", "", offer, nil)
	case model.CaseStatusNeedsInfo:
		requestID := uuid.New()
		if result.Plan != nil {
			var data []byte
			data, err = encodeRequirements(result.Requirements)
			if err != nil {
				return fmt.Errorf("encode automatic requirements: %w", err)
			}
			_, err = q.Exec(ctx, insertV2InfoRequestQuery, requestID, caseID, decisionID, result.PublicReason, string(data))
		} else {
			_, err = q.Exec(ctx, insertInfoRequestQuery, requestID, caseID, decisionID,
				result.PublicReason, result.RequiredFields, result.RequiredDocs)
		}
		if err != nil {
			return fmt.Errorf("insert decision info request: %w", err)
		}
		payload, err = publicstate.NeedsInfo(eventID, caseID, requestID, version+1, now,
			result.PublicReason, result.RequiredFields, result.RequiredDocs)
		if err == nil && result.Plan != nil {
			payload, err = publicstate.WithRequirements(payload, result.Plan, result.Requirements)
		}
	case model.CaseStatusRejected:
		payload, err = publicstate.Decision(eventID, caseID, version+1, now, "REJECT", result.PublicReason, nil, nil)
	case model.CaseStatusManualReview:
		payload = publicstate.UnderReview(eventID, caseID, caseID, version+1, now)
	default:
		return fmt.Errorf("unsupported decision status %q", result.Status)
	}
	if err != nil {
		return fmt.Errorf("encode automatic decision event: %w", err)
	}
	var newVersion int64
	if err = q.QueryRow(ctx, finishCaseQuery, caseID, version, string(result.Status), result.Outcome,
		result.Rules, publicReason).Scan(&newVersion); err != nil {
		return fmt.Errorf("finish automatic decision: %w", err)
	}
	details, err := json.Marshal(map[string]any{"from_status": "DECISION", "to_status": result.Status,
		"outcome": result.Outcome, "decision_id": decisionID, "rules": result.Rules, "score_result_id": scoreID})
	if err != nil {
		return fmt.Errorf("encode automatic decision audit: %w", err)
	}
	if _, err = q.Exec(ctx, insertAuditQuery, uuid.New(), caseID, newVersion, result.Outcome, string(details)); err != nil {
		return fmt.Errorf("audit automatic decision: %w", err)
	}
	if _, err = publicstate.Publish(ctx, q, eventID, caseID.String(), newVersion, payload); err != nil {
		return fmt.Errorf("write automatic decision outbox: %w", err)
	}
	return nil
}

func encodeRequirements(items []*commonpb.Requirement) ([]byte, error) {
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
