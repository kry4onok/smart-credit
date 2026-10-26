package update

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	profileconv "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
)

func (h *Handler) Handle(ctx context.Context, request generated.UpdateCompanyRequestObject) (generated.UpdateCompanyResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	if request.Body == nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnprocessableEntity, "INVALID_COMPANY_PROFILE", "Профиль компании заполнен некорректно.")}, nil
	}
	profile := *request.Body
	if profile.ProfileSchemaVersion == nil && (profile.Revenue12m == nil || profile.Profit12m == nil || profile.Obligations == nil) {
		return problemResponse{domainerror.Problem(ctx, 422, "INVALID_COMPANY_PROFILE", "Укажите финансовые показатели старого профиля.")}, nil
	}
	if !profileconv.ValidCurrencies(profile) {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnprocessableEntity, "INVALID_COMPANY_PROFILE", "Валюта должна быть RUB.")}, nil
	}
	expectedVersion, err := parseIfMatch(request.Params.IfMatch)
	if err != nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_IF_MATCH", "Заголовок If-Match должен содержать версию в двойных кавычках.")}, nil
	}
	company, err := h.usecase.Execute(ctx, client.OrgID, fromRequest(profile), expectedVersion)
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return companyResponse{toResponse(company)}, nil
}

func parseIfMatch(value *string) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	if len(*value) < 3 || !strings.HasPrefix(*value, `"`) || !strings.HasSuffix(*value, `"`) {
		return nil, domain.ErrInvalidIfMatch
	}
	rawVersion := (*value)[1 : len(*value)-1]
	for _, digit := range rawVersion {
		if digit < '0' || digit > '9' {
			return nil, domain.ErrInvalidIfMatch
		}
	}
	version, err := strconv.ParseInt(rawVersion, 10, 64)
	if err != nil || version <= 0 {
		return nil, domain.ErrInvalidIfMatch
	}
	return &version, nil
}

type companyResponse struct{ generated.VersionedCompany }

func (r companyResponse) VisitUpdateCompanyResponse(w http.ResponseWriter) error {
	etag := fmt.Sprintf(`"%d"`, r.Version)
	w.Header().Set("Cache-Control", "no-store")
	return generated.UpdateCompany200JSONResponse{
		Body: r.VersionedCompany,
		Headers: generated.UpdateCompany200ResponseHeaders{
			ETag: &etag,
		},
	}.VisitUpdateCompanyResponse(w)
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitUpdateCompanyResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
