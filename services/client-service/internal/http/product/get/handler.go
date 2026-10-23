package get

import (
	"context"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	producthttp "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product"
)

func (h *Handler) Handle(ctx context.Context, request generated.GetClientProductRequestObject) (generated.GetClientProductResponseObject, error) {
	if _, ok := middleware.Client(ctx); !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	item, err := h.usecase.Execute(ctx, request.ProductId)
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.GetClientProduct200JSONResponse(producthttp.ToAPI(item)), nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitGetClientProductResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
