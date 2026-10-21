package listhandler

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
)

type Handler struct {
	useCase UseCase
}

func New(useCase UseCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Handle(
	ctx context.Context,
	request generated.ListCasesRequestObject,
) (generated.ListCasesResponseObject, error) {
	input := toUseCaseInput(ctx, request)
	output, err := h.useCase.Execute(ctx, input)
	if err != nil {
		if errors.Is(err, listcase.ErrInvalidLimit) ||
			errors.Is(err, listcase.ErrInvalidCursor) ||
			errors.Is(err, listcase.ErrInvalidStatus) ||
			errors.Is(err, listcase.ErrInvalidAssignee) {
			return nil, domainerror.BadRequest("INVALID_LIST_FILTER", err)
		}

		return nil, fmt.Errorf("execute list cases: %w", err)
	}

	return toResponse(output), nil
}

func toUseCaseInput(
	ctx context.Context,
	request generated.ListCasesRequestObject,
) listcase.Input {
	input := listcase.Input{
		Limit:    request.Params.Limit,
		Cursor:   request.Params.Cursor,
		Assignee: request.Params.Assignee,
	}

	if request.Params.Status != nil {
		status := toModelStatus(*request.Params.Status)
		input.Status = &status
	}
	if request.Params.ProductId != nil {
		productID := *request.Params.ProductId
		input.ProductID = &productID
	}
	if currentUserID, ok := authmiddleware.UserID(ctx); ok {
		input.CurrentUserID = currentUserID
	}

	return input
}
