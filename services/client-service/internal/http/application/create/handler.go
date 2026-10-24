package create

import (
	"context"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func (h *Handler) Handle(ctx context.Context, request generated.CreateApplicationRequestObject) (generated.CreateApplicationResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	if request.Params.IdempotencyKey == uuid.Nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key должен быть ненулевым UUID.")}, nil
	}
	if request.Body == nil || request.Body.ProductId == uuid.Nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnprocessableEntity, "INVALID_APPLICATION", "Необходимо выбрать продукт.")}, nil
	}
	application, err := h.usecase.Execute(ctx, model.CreateApplicationParams{
		OrgID: client.OrgID, UserID: client.UserID,
		ProductID: request.Body.ProductId, IdempotencyID: request.Params.IdempotencyKey,
	})
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.CreateApplication201JSONResponse(toResponse(application)), nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitCreateApplicationResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
