package get

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
)

func (h *Handler) Handle(ctx context.Context, request generated.GetApplicationRequestObject) (generated.GetApplicationResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	application, err := h.usecase.Execute(ctx, client.OrgID, request.ApplicationId)
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return applicationResponse{generated.ClientApplication(toResponse(application)), application.Version}, nil
}

type applicationResponse struct {
	generated.ClientApplication
	version int64
}

func (r applicationResponse) VisitGetApplicationResponse(w http.ResponseWriter) error {
	etag := fmt.Sprintf(`"%d"`, r.version)
	w.Header().Set("Cache-Control", "no-store")
	return generated.GetApplication200JSONResponse{
		Body:    r.ClientApplication,
		Headers: generated.GetApplication200ResponseHeaders{ETag: &etag},
	}.VisitGetApplicationResponse(w)
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitGetApplicationResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
