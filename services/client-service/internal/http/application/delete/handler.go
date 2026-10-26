package delete

import (
	"context"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/etag"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func (h *Handler) Handle(ctx context.Context, request generated.DeleteApplicationRequestObject) (generated.DeleteApplicationResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, 401, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	version, err := etag.ParseIfMatch(request.Params.IfMatch)
	if err != nil {
		return problemResponse{domainerror.Problem(ctx, 400, "INVALID_IF_MATCH", "Заголовок If-Match должен содержать версию в двойных кавычках.")}, nil
	}
	if request.ApplicationId == uuid.Nil {
		return problemResponse{domainerror.Problem(ctx, 400, "INVALID_APPLICATION_ID", "ID заявки должен быть ненулевым UUID.")}, nil
	}
	err = h.usecase.Execute(ctx, model.DeleteApplicationParams{OrgID: client.OrgID, ApplicationID: request.ApplicationId, ExpectedVersion: version})
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.DeleteApplication204Response{}, nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitDeleteApplicationResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
