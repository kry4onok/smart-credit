package gethandler

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
)

type Handler struct {
	useCase UseCase
}

func New(useCase UseCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Handle(
	ctx context.Context,
	request generated.GetCaseRequestObject,
) (generated.GetCaseResponseObject, error) {
	input := getcase.Input{CaseID: request.ApplicationId}
	if currentUserID, ok := authmiddleware.UserID(ctx); ok {
		input.CurrentUserID = currentUserID
	}

	output, err := h.useCase.Execute(ctx, input)
	if err != nil {
		if errors.Is(err, getcase.ErrNotFound) {
			return nil, domainerror.NotFound("CASE_NOT_FOUND", err)
		}

		return nil, fmt.Errorf("execute get case: %w", err)
	}

	return toResponse(output), nil
}
