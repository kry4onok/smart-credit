package takehandler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	gethandler "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/case/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/take"
	"github.com/google/uuid"
)

type UseCase interface {
	Execute(context.Context, take.Input) (model.Case, error)
}

type Handler struct{ useCase UseCase }

func New(useCase UseCase) *Handler { return &Handler{useCase: useCase} }

func (h *Handler) Handle(ctx context.Context, request generated.TakeCaseRequestObject) (generated.TakeCaseResponseObject, error) {
	analystID, ok := authmiddleware.AnalystID(ctx)
	if !ok {
		return nil, domainerror.Forbidden("ANALYST_REQUIRED", errors.New("требуется подтверждённый аналитик"))
	}
	if request.ApplicationId == uuid.Nil || request.Params.IdempotencyKey == uuid.Nil {
		return nil, domainerror.BadRequest("INVALID_TAKE_REQUEST", errors.New("ID дела и ключ идемпотентности должны быть UUID"))
	}

	etag := request.Params.IfMatch
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

	caseItem, err := h.useCase.Execute(ctx, take.Input{
		CaseID: request.ApplicationId, AnalystID: analystID,
		ExpectedVersion: version, IdempotencyKey: request.Params.IdempotencyKey,
	})

	if err != nil {
		switch {
		case errors.Is(err, take.ErrNotFound):
			return nil, domainerror.NotFound("CASE_NOT_FOUND", err)
		case errors.Is(err, take.ErrConflict):
			return nil, domainerror.Conflict("CASE_TAKE_CONFLICT", err)
		case errors.Is(err, take.ErrIdempotencyConflict):
			return nil, domainerror.Conflict("IDEMPOTENCY_KEY_CONFLICT", err)
		default:
			return nil, fmt.Errorf("execute take case: %w", err)
		}
	}

	newETag := fmt.Sprintf("\"%d\"", caseItem.Version)

	return generated.TakeCase200JSONResponse{
		Body: gethandler.ToBankCase(getcase.Output{
			Case:           caseItem,
			AllowedActions: []model.CaseAction{model.CaseActionView, model.CaseActionDecide},
		}),
		Headers: generated.TakeCase200ResponseHeaders{ETag: &newETag},
	}, nil
}
