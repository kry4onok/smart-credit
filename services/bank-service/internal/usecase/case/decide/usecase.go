package decide

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

var (
	ErrNotFound            = errors.New("case not found")
	ErrConflict            = errors.New("case version or state conflict")
	ErrForbidden           = errors.New("case is assigned to another analyst")
	ErrIdempotencyConflict = errors.New("idempotency key belongs to a different request")
	ErrValidation          = errors.New("decision violates a business rule")
)

type Offer struct {
	ProductID     uuid.UUID
	AmountKopecks int64
	TermMonths    int
	RateBPS       int
}
type InfoRequest struct {
	Message                               string
	RequiredFields, RequiredDocumentTypes []string
	RequirementIDs                        []string
	V2                                    bool
}
type Input struct {
	CaseID, AnalystID, IdempotencyKey uuid.UUID
	ExpectedVersion                   int64
	Outcome, Comment, PublicReason    string
	Offer                             *Offer
	InfoRequest                       *InfoRequest
	ClientText                        string `json:"-"`
}
type Repository interface {
	Decide(context.Context, Input) (model.Case, error)
	PrepareDecision(context.Context, Input) (Preview, bool)
}
type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Assistant interface {
	Explain(context.Context, Facts) (string, error)
}
type Preview struct {
	RequestedProductName string
	OfferedProductName   string
}
type OfferFacts struct {
	ProductName   string
	AmountKopecks int64
	TermMonths    int
	RateBPS       int
}
type Facts struct {
	ApplicationID string
	Outcome       string
	ProductName   string
	PublicReason  string
	Offer         *OfferFacts
}
type UseCase struct {
	repository   Repository
	transactions TransactionManager
	assistant    Assistant
}

func New(r Repository, t TransactionManager, assistant Assistant) *UseCase {
	return &UseCase{repository: r, transactions: t, assistant: assistant}
}

func (u *UseCase) Execute(ctx context.Context, input Input) (model.Case, error) {
	if err := Validate(input); err != nil {
		return model.Case{}, err
	}
	if input.Outcome == "APPROVE" && strings.TrimSpace(input.PublicReason) == "" {
		input.ClientText = "По вашей заявке принято решение об одобрении."
	}
	if u.assistant != nil && input.Outcome != "REQUEST_INFO" {
		if preview, ready := u.repository.PrepareDecision(ctx, input); ready {
			facts := Facts{
				ApplicationID: input.CaseID.String(), Outcome: input.Outcome,
				ProductName: preview.RequestedProductName, PublicReason: input.PublicReason,
			}
			if input.Offer != nil {
				facts.Offer = &OfferFacts{
					ProductName:   preview.OfferedProductName,
					AmountKopecks: input.Offer.AmountKopecks,
					TermMonths:    input.Offer.TermMonths, RateBPS: input.Offer.RateBPS,
				}
			}
			text, err := u.assistant.Explain(ctx, facts)
			if err == nil {
				text = strings.TrimSpace(text)
				if text != "" && utf8.RuneCountInString(text) <= 1000 {
					input.ClientText = text
				}
			}
		}
	}
	var result model.Case
	err := u.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		var e error
		result, e = u.repository.Decide(txCtx, input)
		return e
	})
	if err != nil {
		return model.Case{}, fmt.Errorf("decide case: %w", err)
	}
	return result, nil
}

func Validate(in Input) error {
	bad := func(message string) error { return fmt.Errorf("%w: %s", ErrValidation, message) }
	if in.CaseID == uuid.Nil || in.AnalystID == uuid.Nil || in.IdempotencyKey == uuid.Nil || in.ExpectedVersion < 1 {
		return bad("invalid case, analyst, key or version")
	}
	if strings.TrimSpace(in.Comment) == "" || utf8.RuneCountInString(in.Comment) > 2000 {
		return bad("comment must contain 1–2000 characters")
	}
	if utf8.RuneCountInString(in.PublicReason) > 500 {
		return bad("public_reason exceeds 500 characters")
	}
	switch in.Outcome {
	case "APPROVE", "COUNTER":
		if in.Offer == nil || in.InfoRequest != nil {
			return bad("offer is required and info_request is forbidden")
		}
		if in.Offer.ProductID == uuid.Nil || in.Offer.AmountKopecks <= 0 || in.Offer.TermMonths <= 0 || in.Offer.TermMonths > math.MaxInt32 || in.Offer.RateBPS < 0 || in.Offer.RateBPS > math.MaxInt32 {
			return bad("invalid offer terms")
		}
		if in.Outcome == "COUNTER" && strings.TrimSpace(in.PublicReason) == "" {
			return bad("public_reason is required")
		}
	case "REQUEST_INFO":
		if in.Offer != nil || in.InfoRequest == nil {
			return bad("info_request is required and offer is forbidden")
		}
		if strings.TrimSpace(in.InfoRequest.Message) == "" {
			return bad("info_request requires message and missing items")
		}
		if in.InfoRequest.V2 {
			if len(in.InfoRequest.RequiredFields)+len(in.InfoRequest.RequiredDocumentTypes) != 0 || len(in.InfoRequest.RequirementIDs) == 0 {
				return bad("requirement_ids must be nonempty and cannot be mixed with legacy fields")
			}
			return nil
		}
		if len(in.InfoRequest.RequiredFields)+len(in.InfoRequest.RequiredDocumentTypes) == 0 {
			return bad("info_request requires missing items")
		}
		fields := map[string]bool{}
		docs := map[string]bool{}
		for _, v := range in.InfoRequest.RequiredFields {
			switch v {
			case "COMPANY_REVENUE_12M", "COMPANY_PROFIT_12M", "COMPANY_OBLIGATIONS", "APPLICATION_AMOUNT", "APPLICATION_TERM_MONTHS", "APPLICATION_PURPOSE":
			default:
				return bad("unknown required field")
			}
			if fields[v] {
				return bad("duplicate required field")
			}
			fields[v] = true
		}
		for _, v := range in.InfoRequest.RequiredDocumentTypes {
			switch v {
			case "BANK_STATEMENT", "BUSINESS_PLAN":
			default:
				return bad("unknown document type")
			}
			if docs[v] {
				return bad("duplicate document type")
			}
			docs[v] = true
		}
	case "REJECT":
		if in.Offer != nil || in.InfoRequest != nil || strings.TrimSpace(in.PublicReason) == "" {
			return bad("reject requires public_reason and forbids offer and info_request")
		}
	default:
		return bad("unknown outcome")
	}
	return nil
}
