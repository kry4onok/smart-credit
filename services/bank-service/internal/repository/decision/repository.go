package decisionrepository

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
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/decision"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	var found bool
	err := r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		q := transaction.QueryerFromContext(ctx, r.db)
		var caseID, productID uuid.UUID
		var version, productVersion int64
		var snapshot validation.Snapshot
		err := q.QueryRow(ctx, claimCaseQuery).Scan(&caseID, &version, &productID, &productVersion,
			&snapshot.CompanyJSON, &snapshot.DocumentsJSON, &snapshot.AmountKopecks,
			&snapshot.TermMonths, &snapshot.PurposeTag)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim decision case: %w", err)
		}
		found = true
		input := decision.Input{Snapshot: snapshot, Now: time.Now().UTC()}
		input.Product, err = loadProduct(ctx, q, productID, productVersion)
		if err != nil {
			return err
		}
		var scoreID *int64
		var grade string
		err = q.QueryRow(ctx, loadScoreQuery, caseID).Scan(&scoreID, &grade)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("load decision score: %w", err)
		}
		input.ScoreFound = err == nil && scoreID != nil
		input.Grade = grade
		var companyOutcome, companyStatus, creditOutcome *string
		var overdue *int
		if err = q.QueryRow(ctx, loadChecksQuery, caseID).Scan(&companyOutcome, &companyStatus, &creditOutcome, &overdue); err != nil {
			return fmt.Errorf("load decision checks: %w", err)
		}
		input.CompanyOK = companyOutcome != nil && *companyOutcome == "SUCCESS" && companyStatus != nil
		input.CreditOK = creditOutcome != nil && *creditOutcome == "SUCCESS" && overdue != nil
		if companyStatus != nil {
			input.CompanyStatus = *companyStatus
		}
		if overdue != nil {
			input.OverdueDays = *overdue
		}
		result := decision.Evaluate(input)
		return complete(ctx, q, caseID, productID, version, scoreID, input, result)
	})
	return found, err
}

func loadProduct(ctx context.Context, q transaction.Queryer, id uuid.UUID, version int64) (decision.Product, error) {
	var p decision.Product
	err := q.QueryRow(ctx, loadProductQuery, id, version).Scan(&p.Name, &p.Version,
		&p.Purposes, &p.Documents, &p.MinAmount, &p.MaxAmount, &p.MinTerm, &p.MaxTerm,
		&p.MinAgeMonths, &p.MaxAmountRevenueBP, &p.MaxOverdueDays,
		&p.GradeARateBP, &p.GradeBRateBP, &p.OfferTTLHours)
	if errors.Is(err, pgx.ErrNoRows) {
		return decision.Product{}, nil
	}
	if err != nil {
		return decision.Product{}, fmt.Errorf("load decision product: %w", err)
	}
	p.Found = true
	return p, nil
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
		if _, err = q.Exec(ctx, insertInfoRequestQuery, requestID, caseID, decisionID,
			result.PublicReason, result.RequiredFields, result.RequiredDocs); err != nil {
			return fmt.Errorf("insert decision info request: %w", err)
		}
		payload, err = publicstate.NeedsInfo(eventID, caseID, requestID, version+1, now,
			result.PublicReason, result.RequiredFields, result.RequiredDocs)
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
	if _, err = q.Exec(ctx, insertOutboxQuery, eventID, caseID.String(), newVersion, payload); err != nil {
		return fmt.Errorf("write automatic decision outbox: %w", err)
	}
	return nil
}
