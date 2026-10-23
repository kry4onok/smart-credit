package list

import (
	"context"
	"errors"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
)

func (h *Handler) Handle(ctx context.Context, request generated.ListApplicationsRequestObject) (generated.ListApplicationsResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	input := listapplication.Input{OrgID: client.OrgID, Limit: request.Params.Limit, Cursor: request.Params.Cursor}
	if request.Params.Status != nil {
		status := string(*request.Params.Status)
		input.Status = &status
	}
	output, err := h.usecase.Execute(ctx, input)
	if err != nil {
		if errors.Is(err, listapplication.ErrInvalidLimit) || errors.Is(err, listapplication.ErrInvalidCursor) || errors.Is(err, listapplication.ErrInvalidStatus) {
			return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_LIST_FILTER", "Параметры списка заявок заполнены некорректно.")}, nil
		}
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return toResponse(output), nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitListApplicationsResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
