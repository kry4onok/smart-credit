package summaryhandler

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	summarycase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/summary"
	"github.com/google/uuid"
)

type UseCase interface {
	Execute(context.Context, uuid.UUID) (summarycase.Output, error)
}

type Handler struct {
	useCase UseCase
}

func New(useCase UseCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Handle(ctx context.Context, request generated.SummarizeCaseRequestObject) (generated.SummarizeCaseResponseObject, error) {
	if _, ok := authmiddleware.UserID(ctx); !ok {
		return nil, domainerror.Forbidden("EMPLOYEE_REQUIRED", errors.New("требуется подтверждённый сотрудник банка"))
	}
	if request.ApplicationId == uuid.Nil {
		return nil, domainerror.BadRequest("INVALID_CASE_ID", errors.New("ID дела должен быть UUID"))
	}
	result, err := h.useCase.Execute(ctx, request.ApplicationId)
	if err != nil {
		if errors.Is(err, summarycase.ErrNotFound) {
			return nil, domainerror.NotFound("CASE_NOT_FOUND", err)
		}
		return nil, fmt.Errorf("execute case summary: %w", err)
	}
	return generated.SummarizeCase200JSONResponse{
		Text: result.Text, Source: generated.SummarizeCase200JSONResponseBodySource(result.Source),
	}, nil
}
