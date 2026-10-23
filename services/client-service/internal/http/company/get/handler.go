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

type Handler struct{ usecase Usecase }

func New(usecase Usecase) *Handler { return &Handler{usecase: usecase} }

func (h *Handler) Handle(ctx context.Context, _ generated.GetCompanyRequestObject) (generated.GetCompanyResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	company, err := h.usecase.Execute(ctx, client.OrgID)
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return companyResponse{convert(company)}, nil
}

// The generated response has no ETag field; keep generated code untouched.
type companyResponse struct{ generated.VersionedCompany }

func (r companyResponse) VisitGetCompanyResponse(w http.ResponseWriter) error {
	w.Header().Set("ETag", fmt.Sprintf(`"%d"`, r.Version))
	w.Header().Set("Cache-Control", "no-store")
	return generated.GetCompany200JSONResponse(r.VersionedCompany).VisitGetCompanyResponse(w)
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitGetCompanyResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
