package update

import (
	"context"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/etag"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	applicationget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

func (h *Handler) Handle(ctx context.Context, request generated.UpdateApplicationRequestObject) (generated.UpdateApplicationResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	expectedVersion, err := parseIfMatch(request.Params.IfMatch)
	if err != nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_IF_MATCH", "Заголовок If-Match должен содержать версию в двойных кавычках.")}, nil
	}
	if request.Body == nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnprocessableEntity, "INVALID_APPLICATION_FORM", "Форма заявки заполнена некорректно.")}, nil
	}
	patch := model.ApplicationPatch{TermMonths: request.Body.TermMonths, PurposeDescription: request.Body.PurposeDescription}
	if request.Body.Company != nil {
		if !profile.ValidCurrencies(*request.Body.Company) {
			return problemResponse{domainerror.Problem(ctx, 422, "INVALID_COMPANY_PROFILE", "Валюта должна быть RUB.")}, nil
		}
		c := profile.FromHTTP(*request.Body.Company)
		patch.Company = &c
	}
	if request.Body.Purpose != nil {
		purpose := string(*request.Body.Purpose)
		patch.Purpose = &purpose
	}
	if request.Body.Amount != nil {
		amount := request.Body.Amount.AmountKopecks
		patch.AmountKopecks = &amount
		patch.AmountCurrency = string(request.Body.Amount.Currency)
	}
	application, err := h.usecase.Execute(ctx, model.UpdateApplicationParams{
		OrgID: client.OrgID, ApplicationID: request.ApplicationId,
		Patch: patch, ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.UpdateApplication200JSONResponse(applicationget.ToResponse(application)), nil
}

func parseIfMatch(value string) (int64, error) { return etag.ParseIfMatch(value) }

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitUpdateApplicationResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
