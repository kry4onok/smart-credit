package list

import (
	"context"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
)

func (h *Handler) Handle(ctx context.Context, request generated.ListClientProductsRequestObject) (generated.ListClientProductsResponseObject, error) {
	if _, ok := middleware.Client(ctx); !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}

	var purpose *string
	if request.Params.Purpose != nil {
		value := string(*request.Params.Purpose)
		if !validPurpose(value) {
			return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_PURPOSE", "Параметр purpose содержит недопустимое значение.")}, nil
		}
		purpose = &value
	}

	catalog, err := h.usecase.Execute(ctx, purpose)
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return toResponse(catalog), nil
}

func validPurpose(value string) bool {
	switch common.PurposeTag(value) {
	case common.WORKINGCAPITAL, common.EQUIPMENT, common.EXPANSION:
		return true
	default:
		return false
	}
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitListClientProductsResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
