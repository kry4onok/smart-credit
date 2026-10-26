package decidehandler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	gethandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/decide"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	"github.com/google/uuid"
)

type UseCase interface {
	Execute(context.Context, decide.Input) (model.Case, error)
}
type Handler struct{ useCase UseCase }

func New(u UseCase) *Handler { return &Handler{u} }
func (h *Handler) Handle(ctx context.Context, r generated.DecideCaseRequestObject) (generated.DecideCaseResponseObject, error) {
	analystID, ok := authmiddleware.AnalystID(ctx)
	if !ok {
		return nil, domainerror.Forbidden("ANALYST_REQUIRED", errors.New("требуется подтверждённый аналитик"))
	}
	if r.ApplicationId == uuid.Nil || r.Params.IdempotencyKey == uuid.Nil || r.Body == nil {
		return nil, domainerror.BadRequest("INVALID_DECISION_REQUEST", errors.New("ID дела, ключ и тело обязательны"))
	}
	etag := r.Params.IfMatch
	if len(etag) < 3 || !strings.HasPrefix(etag, "\"") || !strings.HasSuffix(etag, "\"") {
		return nil, domainerror.BadRequest("INVALID_IF_MATCH", errors.New("If-Match должен содержать версию в двойных кавычках"))
	}
	for _, digit := range etag[1 : len(etag)-1] {
		if digit < '0' || digit > '9' {
			return nil, domainerror.BadRequest("INVALID_IF_MATCH", errors.New("If-Match должен содержать только цифры"))
		}
	}
	version, err := strconv.ParseInt(etag[1:len(etag)-1], 10, 64)
	if err != nil || version < 1 {
		return nil, domainerror.BadRequest("INVALID_IF_MATCH", errors.New("If-Match должен содержать положительную версию"))
	}
	input := decide.Input{CaseID: r.ApplicationId, AnalystID: analystID, IdempotencyKey: r.Params.IdempotencyKey, ExpectedVersion: version, Outcome: string(r.Body.Outcome), Comment: r.Body.Comment}
	if r.Body.PublicReason != nil {
		input.PublicReason = *r.Body.PublicReason
	}
	if r.Body.Offer != nil {
		input.Offer = &decide.Offer{ProductID: r.Body.Offer.ProductId, AmountKopecks: r.Body.Offer.Amount.AmountKopecks, TermMonths: r.Body.Offer.TermMonths, RateBPS: r.Body.Offer.RateBps}
		if r.Body.Offer.Amount.Currency != "RUB" {
			return nil, domainerror.BadRequest("INVALID_CURRENCY", errors.New("currency must be RUB"))
		}
	}
	if r.Body.InfoRequest != nil {
		draft := r.Body.InfoRequest
		info := &decide.InfoRequest{Message: draft.Message, V2: draft.RequirementIds != nil}
		if draft.RequirementIds != nil {
			info.RequirementIDs = append([]string(nil), (*draft.RequirementIds)...)
		}
		for _, f := range draft.RequiredFields {
			info.RequiredFields = append(info.RequiredFields, string(f))
		}
		for _, d := range draft.RequiredDocumentTypes {
			info.RequiredDocumentTypes = append(info.RequiredDocumentTypes, string(d))
		}
		input.InfoRequest = info
	}
	result, err := h.useCase.Execute(ctx, input)
	if err != nil {
		var ruleError *requirements.RuleError
		switch {
		case errors.As(err, &ruleError):
			return nil, domainerror.Unprocessable(ruleError.Code, err)
		case errors.Is(err, decide.ErrNotFound):
			return nil, domainerror.NotFound("CASE_NOT_FOUND", err)
		case errors.Is(err, decide.ErrForbidden):
			return nil, domainerror.Forbidden("CASE_NOT_ASSIGNED", err)
		case errors.Is(err, decide.ErrConflict):
			return nil, domainerror.Conflict("CASE_DECISION_CONFLICT", err)
		case errors.Is(err, decide.ErrIdempotencyConflict):
			return nil, domainerror.Conflict("IDEMPOTENCY_KEY_CONFLICT", err)
		case errors.Is(err, decide.ErrValidation):
			return nil, &domainerror.Error{Status: 422, Code: "DECISION_RULE_VIOLATION", Title: "Нарушено бизнес-правило", Detail: err.Error()}
		default:
			return nil, fmt.Errorf("execute decision: %w", err)
		}
	}
	return generated.DecideCase200JSONResponse(gethandler.ToBankCase(getcase.Output{Case: result, AllowedActions: []model.CaseAction{model.CaseActionView}})), nil
}
