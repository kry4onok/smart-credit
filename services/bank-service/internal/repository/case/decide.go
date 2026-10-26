package caserepository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/decide"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (r *Repository) PrepareDecision(ctx context.Context, in decide.Input) (decide.Preview, bool) {
	q := transaction.QueryerFromContext(ctx, r.db)
	var status model.CaseStatus
	var assignee *uuid.UUID
	var version int64
	var company, documents []byte
	var purpose, requestedProductName string
	var dossier decisionDossier
	err := q.QueryRow(ctx, previewDecisionCase, in.CaseID).Scan(
		&status, &assignee, &version, &company, &documents, &purpose, &requestedProductName,
		&dossier.referenceDate, &dossier.policy, &dossier.purposes, &dossier.documents,
		&dossier.minAmount, &dossier.maxAmount, &dossier.minTerm, &dossier.maxTerm, &dossier.amount, &dossier.term,
	)
	if err != nil || assignee == nil || *assignee != in.AnalystID ||
		status != model.CaseStatusManualReview || version != in.ExpectedVersion {
		return decide.Preview{}, false
	}
	preview := decide.Preview{RequestedProductName: requestedProductName}
	if in.Outcome == "APPROVE" || in.Outcome == "COUNTER" {
		product, err := r.validateDecisionOffer(ctx, in, company, documents, purpose, dossier, time.Now().UTC())
		if err != nil {
			return decide.Preview{}, false
		}
		preview.OfferedProductName = product.name
	}
	return preview, true
}

type decisionProduct struct {
	version                                                                  int64
	name                                                                     string
	purposes, documents                                                      []string
	minAmount, maxAmount                                                     int64
	minTerm, maxTerm, gradeARate, gradeBRate, minAge, ratio, maxOverdue, ttl int
	policy                                                                   string
}

type decisionDossier struct {
	referenceDate                *time.Time
	policy                       string
	purposes, documents          []string
	minAmount, maxAmount, amount int64
	minTerm, maxTerm, term       int32
}

func (d decisionDossier) product() validation.Product {
	return validation.Product{Found: true, PolicyVersion: d.policy, PurposeTags: d.purposes,
		RequiredDocumentTypes: d.documents, MinAmountKopecks: d.minAmount, MaxAmountKopecks: d.maxAmount,
		MinTermMonths: d.minTerm, MaxTermMonths: d.maxTerm}
}

func (r *Repository) Decide(ctx context.Context, in decide.Input) (model.Case, error) {
	q := transaction.QueryerFromContext(ctx, r.db)
	serialized, err := json.Marshal(in)
	if err != nil {
		return model.Case{}, fmt.Errorf("encode decision request: %w", err)
	}

	hash := sha256.Sum256(serialized)
	var reserved int
	err = q.QueryRow(ctx, reserveDecisionKey, in.AnalystID, decisionRoute, in.IdempotencyKey, hash[:]).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var oldHash, body []byte
		if err = q.QueryRow(ctx, readDecisionKey, in.AnalystID, decisionRoute, in.IdempotencyKey).Scan(&oldHash, &body); err != nil {
			return model.Case{}, fmt.Errorf("read decision replay: %w", err)
		}
		if !bytes.Equal(hash[:], oldHash) {
			return model.Case{}, decide.ErrIdempotencyConflict
		}
		var result model.Case
		if err = json.Unmarshal(body, &result); err != nil {
			return model.Case{}, fmt.Errorf("decode decision replay: %w", err)
		}

		return result, nil
	}
	if err != nil {
		return model.Case{}, fmt.Errorf("reserve decision key: %w", err)
	}

	var status model.CaseStatus
	var assignee *uuid.UUID
	var version int64
	var company, documents []byte
	var purpose string
	var dossier decisionDossier

	err = q.QueryRow(ctx, lockDecisionCase, in.CaseID).Scan(&status, &assignee, &version, &company, &documents, &purpose,
		&dossier.referenceDate, &dossier.policy, &dossier.purposes, &dossier.documents,
		&dossier.minAmount, &dossier.maxAmount, &dossier.minTerm, &dossier.maxTerm, &dossier.amount, &dossier.term)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Case{}, decide.ErrNotFound
	}
	if err != nil {
		return model.Case{}, fmt.Errorf("lock decision case: %w", err)
	}
	if assignee == nil || *assignee != in.AnalystID {
		return model.Case{}, decide.ErrForbidden
	}
	if status != model.CaseStatusManualReview || version != in.ExpectedVersion {
		return model.Case{}, decide.ErrConflict
	}

	now := time.Now().UTC()
	decisionID := uuid.New()
	clientText := in.PublicReason
	if strings.TrimSpace(in.ClientText) != "" {
		clientText = in.ClientText
	}
	var productID any
	var productVersion any
	var ruleVersion any
	var offer *commonpb.Offer
	var request *commonpb.InfoRequest
	var publicReason any
	var nextStatus model.CaseStatus

	switch in.Outcome {
	case "APPROVE", "COUNTER":
		p, e := r.validateDecisionOffer(ctx, in, company, documents, purpose, dossier, now)
		if e != nil {
			return model.Case{}, e
		}
		productID = in.Offer.ProductID
		productVersion = p.version
		ruleVersion = p.version
		kind := model.CaseStatusOffer
		offerKind := commonpb.OfferKind_OFFER_KIND_OFFER
		if in.Outcome == "COUNTER" {
			kind = model.CaseStatusCounterOffer
			offerKind = commonpb.OfferKind_OFFER_KIND_COUNTER_OFFER
		}
		publicReason = nullableReason(clientText)
		nextStatus = kind
		offerID := uuid.New()
		expires := now.Add(time.Duration(p.ttl) * time.Hour)
		var explanation string
		if in.Outcome == "COUNTER" || in.ClientText != "" {
			explanation = clientText
		}
		_, err = q.Exec(ctx, insertDecision, decisionID, in.CaseID, in.AnalystID, in.Outcome, in.Comment, nullableReason(clientText), productID, productVersion, ruleVersion)
		if err != nil {
			return model.Case{}, fmt.Errorf("insert decision: %w", err)
		}
		_, err = q.Exec(ctx, insertDecisionOffer, offerID, in.CaseID, decisionID, string(kind), in.Offer.ProductID, p.version, p.name, in.Offer.AmountKopecks, in.Offer.TermMonths, in.Offer.RateBPS, explanation, expires)
		if err != nil {
			return model.Case{}, fmt.Errorf("insert offer: %w", err)
		}
		offer = &commonpb.Offer{OfferId: offerID.String(), Kind: offerKind, Amount: &commonpb.Money{AmountKopecks: in.Offer.AmountKopecks, Currency: "RUB"}, TermMonths: int32(in.Offer.TermMonths), RateBps: int32(in.Offer.RateBPS), ExpiresAt: timestamppb.New(expires), Explanation: explanation, ProductId: in.Offer.ProductID.String(), ProductVersion: p.version, ProductName: p.name}
	case "REQUEST_INFO":
		if in.InfoRequest.V2 != (dossier.policy != "") {
			return model.Case{}, &requirements.RuleError{Code: requirements.UnknownRequirement}
		}
		nextStatus = model.CaseStatusNeedsInfo
		publicReason = in.InfoRequest.Message
		requestID := uuid.New()
		var selected []*commonpb.Requirement
		var plan *commonpb.RequirementsPlan
		var data []byte
		if in.InfoRequest.V2 {
			checked := validation.Evaluate(validation.Snapshot{CompanyJSON: company, DocumentsJSON: documents, ReferenceDate: dossier.referenceDate,
				AmountKopecks: dossier.amount, TermMonths: dossier.term, PurposeTag: purpose}, dossier.product())
			plan = checked.Plan
			selected, err = requirements.SelectRequest(plan, in.InfoRequest.RequirementIDs)
			if err != nil {
				return model.Case{}, err
			}
			data, err = encodeDecisionRequirements(selected)
			if err != nil {
				return model.Case{}, err
			}
		}
		_, err = q.Exec(ctx, insertDecision, decisionID, in.CaseID, in.AnalystID, in.Outcome, in.Comment, nil, nil, nil, nil)
		if err != nil {
			return model.Case{}, fmt.Errorf("insert decision: %w", err)
		}
		if in.InfoRequest.V2 {
			_, err = q.Exec(ctx, insertDecisionV2Info, requestID, in.CaseID, decisionID, in.InfoRequest.Message, string(data))
		} else {
			_, err = q.Exec(ctx, insertDecisionInfo, requestID, in.CaseID, decisionID, in.InfoRequest.Message, in.InfoRequest.RequiredFields, in.InfoRequest.RequiredDocumentTypes)
		}
		if err != nil {
			return model.Case{}, fmt.Errorf("insert info request: %w", err)
		}
		request = &commonpb.InfoRequest{RequestId: requestID.String(), Message: in.InfoRequest.Message, CreatedAt: timestamppb.New(now)}
		for _, field := range in.InfoRequest.RequiredFields {
			request.RequiredFields = append(request.RequiredFields, commonpb.InfoField(commonpb.InfoField_value["INFO_FIELD_"+field]))
		}
		for _, doc := range in.InfoRequest.RequiredDocumentTypes {
			request.RequiredDocumentTypes = append(request.RequiredDocumentTypes, commonpb.DocumentType(commonpb.DocumentType_value["DOCUMENT_TYPE_"+doc]))
		}
		if in.InfoRequest.V2 {
			request.Requirements = selected
		}
	case "REJECT":
		nextStatus = model.CaseStatusRejected
		publicReason = clientText
		_, err = q.Exec(ctx, insertDecision, decisionID, in.CaseID, in.AnalystID, in.Outcome, in.Comment, clientText, nil, nil, nil)
		if err != nil {
			return model.Case{}, fmt.Errorf("insert decision: %w", err)
		}
	}
	_, err = q.Exec(ctx, updateDecisionCase, in.CaseID, string(nextStatus), in.Outcome, publicReason)
	if err != nil {
		return model.Case{}, fmt.Errorf("update decision case: %w", err)
	}
	details, _ := json.Marshal(map[string]any{"from_status": "MANUAL_REVIEW", "to_status": nextStatus, "outcome": in.Outcome, "decision_id": decisionID, "comment": in.Comment})
	_, err = q.Exec(ctx, insertDecisionAudit, uuid.New(), in.CaseID, version+1, in.AnalystID, in.Outcome, string(details))
	if err != nil {
		return model.Case{}, fmt.Errorf("audit decision: %w", err)
	}
	eventID := uuid.New()
	reason := ""
	if publicReason != nil {
		reason = publicReason.(string)
	}
	payload, err := publicstate.Decision(eventID, in.CaseID, version+1, now, in.Outcome, reason, offer, request)
	if err != nil {
		return model.Case{}, fmt.Errorf("encode decision event: %w", err)
	}
	if request != nil && in.InfoRequest.V2 {
		checked := validation.Evaluate(validation.Snapshot{CompanyJSON: company, DocumentsJSON: documents, ReferenceDate: dossier.referenceDate,
			AmountKopecks: dossier.amount, TermMonths: dossier.term, PurposeTag: purpose}, dossier.product())
		payload, err = publicstate.WithRequirements(payload, checked.Plan, request.Requirements)
		if err != nil {
			return model.Case{}, fmt.Errorf("add decision requirements: %w", err)
		}
	}
	_, err = q.Exec(ctx, insertDecisionOutbox, eventID, in.CaseID.String(), version+1, payload)
	if err != nil {
		return model.Case{}, fmt.Errorf("write decision outbox: %w", err)
	}
	result, found, err := r.GetCase(ctx, in.CaseID)
	if err != nil {
		return model.Case{}, err
	}
	if !found {
		return model.Case{}, decide.ErrNotFound
	}
	body, err := json.Marshal(result)
	if err != nil {
		return model.Case{}, fmt.Errorf("encode decision response: %w", err)
	}
	_, err = q.Exec(ctx, saveDecisionKey, in.AnalystID, decisionRoute, in.IdempotencyKey, body, fmt.Sprintf("\"%d\"", result.Version), in.CaseID)
	if err != nil {
		return model.Case{}, fmt.Errorf("save decision response: %w", err)
	}
	return result, nil
}

func nullableReason(reason string) any {
	if reason == "" {
		return nil
	}
	return reason
}

func (r *Repository) validateDecisionOffer(ctx context.Context, in decide.Input, company, documents []byte, purpose string, dossier decisionDossier, now time.Time) (decisionProduct, error) {
	q := transaction.QueryerFromContext(ctx, r.db)
	var p decisionProduct
	err := q.QueryRow(ctx, loadDecisionProduct, in.Offer.ProductID).Scan(&p.version, &p.name, &p.purposes, &p.documents, &p.minAmount, &p.maxAmount, &p.minTerm, &p.maxTerm, &p.gradeARate, &p.gradeBRate, &p.minAge, &p.ratio, &p.maxOverdue, &p.ttl, &p.policy)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, fmt.Errorf("%w: offer product is inactive or has no rules", decide.ErrValidation)
	}
	if err != nil {
		return p, fmt.Errorf("load offer product: %w", err)
	}
	if in.Offer.AmountKopecks < p.minAmount || in.Offer.AmountKopecks > p.maxAmount || in.Offer.TermMonths < p.minTerm || in.Offer.TermMonths > p.maxTerm {
		return p, fmt.Errorf("%w: offer outside product amount or term range", decide.ErrValidation)
	}
	var grade string
	gradeErr := q.QueryRow(ctx, latestDecisionGrade, in.CaseID).Scan(&grade)
	if gradeErr != nil && !errors.Is(gradeErr, pgx.ErrNoRows) {
		return p, fmt.Errorf("load latest score grade: %w", gradeErr)
	}
	if grade == "A" && in.Offer.RateBPS < p.gradeARate || grade == "B" && in.Offer.RateBPS < p.gradeBRate {
		return p, fmt.Errorf("%w: offer rate below grade rule", decide.ErrValidation)
	}

	var companyStatus *string
	statusErr := q.QueryRow(ctx, latestDecisionCompanyStatus, in.CaseID).Scan(&companyStatus)
	if statusErr != nil && !errors.Is(statusErr, pgx.ErrNoRows) {
		return p, fmt.Errorf("load company status: %w", statusErr)
	}
	if errors.Is(statusErr, pgx.ErrNoRows) || companyStatus == nil {
		return p, fmt.Errorf("%w: successful company check is required", decide.ErrValidation)
	}
	if companyStatus != nil && (*companyStatus == "COMPANY_STATUS_LIQUIDATING" || *companyStatus == "COMPANY_STATUS_LIQUIDATED" || *companyStatus == "LIQUIDATING" || *companyStatus == "LIQUIDATED") {
		return p, fmt.Errorf("%w: company status forbids an offer", decide.ErrValidation)
	}

	if dossier.policy != p.policy {
		return p, fmt.Errorf("%w: counter product policy differs from requested product", decide.ErrValidation)
	}
	outcome := validation.Evaluate(validation.Snapshot{CompanyJSON: company, DocumentsJSON: documents, ReferenceDate: dossier.referenceDate, AmountKopecks: in.Offer.AmountKopecks, TermMonths: int32(in.Offer.TermMonths), PurposeTag: purpose}, validation.Product{Found: true, PolicyVersion: p.policy, PurposeTags: p.purposes, RequiredDocumentTypes: p.documents, MinAmountKopecks: p.minAmount, MaxAmountKopecks: p.maxAmount, MinTermMonths: int32(p.minTerm), MaxTermMonths: int32(p.maxTerm)})
	if outcome.Status != model.CaseStatusEnrichment {
		return p, fmt.Errorf("%w: offer requires fields %v and documents %v (%s)", decide.ErrValidation, outcome.RequiredFields, outcome.RequiredDocumentTypes, outcome.Reason)
	}

	var snapshot struct {
		RegisteredAt string `json:"registered_at"`
		Revenue      *struct {
			Amount json.RawMessage `json:"amount_kopecks"`
		} `json:"revenue_12m"`
	}

	if err = json.Unmarshal(company, &snapshot); err != nil {
		return p, fmt.Errorf("%w: invalid company snapshot", decide.ErrValidation)
	}
	registered, err := time.Parse("2006-01-02", snapshot.RegisteredAt)
	if err != nil {
		return p, fmt.Errorf("%w: invalid company registration date", decide.ErrValidation)
	}
	if registered.AddDate(0, p.minAge, 0).After(now) {
		return p, fmt.Errorf("%w: company age below product minimum", decide.ErrValidation)
	}
	if snapshot.Revenue == nil || len(snapshot.Revenue.Amount) == 0 {
		return p, fmt.Errorf("%w: revenue is required", decide.ErrValidation)
	}
	revenue, err := strconv.ParseInt(strings.Trim(string(snapshot.Revenue.Amount), `"`), 10, 64)
	if err != nil || revenue <= 0 {
		return p, fmt.Errorf("%w: revenue must be positive", decide.ErrValidation)
	}

	amount := new(big.Int).Mul(big.NewInt(in.Offer.AmountKopecks), big.NewInt(10000))
	limit := new(big.Int).Mul(big.NewInt(revenue), big.NewInt(int64(p.ratio)))
	if amount.Cmp(limit) > 0 {
		return p, fmt.Errorf("%w: amount exceeds revenue limit", decide.ErrValidation)
	}
	var overdue *int

	err = q.QueryRow(ctx, latestDecisionOverdue, in.CaseID).Scan(&overdue)
	if errors.Is(err, pgx.ErrNoRows) || overdue == nil {
		return p, fmt.Errorf("%w: successful credit history check is required", decide.ErrValidation)
	}
	if err == nil && *overdue > p.maxOverdue {
		return p, fmt.Errorf("%w: overdue days exceed product limit", decide.ErrValidation)
	}

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, fmt.Errorf("load credit history: %w", err)
	}
	return p, nil
}

func encodeDecisionRequirements(items []*commonpb.Requirement) ([]byte, error) {
	raw := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(item)
		if err != nil {
			return nil, err
		}
		raw = append(raw, data)
	}
	return json.Marshal(raw)
}
