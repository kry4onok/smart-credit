package update

import (
	"context"
	"net/http"
	"strconv"
	"strings"

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
	patch := model.ApplicationPatch{TermMonths: request.Body.TermMonths}
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

func parseIfMatch(value string) (int64, error) {
	if len(value) < 3 || !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) {
		return 0, strconv.ErrSyntax
	}
	rawVersion := value[1 : len(value)-1]
	for _, digit := range rawVersion {
		if digit < '0' || digit > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	version, err := strconv.ParseInt(rawVersion, 10, 64)
	if err != nil || version <= 0 {
		return 0, strconv.ErrSyntax
	}
	return version, nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitUpdateApplicationResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
