package info

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func (h *Handler) Handle(ctx context.Context, request generated.ProvideInfoRequestObject) (generated.ProvideInfoResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	if request.ApplicationId == uuid.Nil || request.Params.IdempotencyKey == uuid.Nil ||
		request.Body == nil || request.Body.RequestId == uuid.Nil {
		if request.Body == nil {
			return problemResponse{domainerror.Problem(ctx, http.StatusUnprocessableEntity, "INVALID_INFO_RESPONSE", "Ответ на запрос банка заполнен некорректно.")}, nil
		}
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_INFO_REQUEST", "ID заявки, request_id и Idempotency-Key должны быть ненулевыми UUID.")}, nil
	}

	hashInput, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(hashInput)
	params := model.ProvideInfoParams{
		OrgID: client.OrgID, UserID: client.UserID, ApplicationID: request.ApplicationId,
		RequestID: request.Body.RequestId, IdempotencyID: request.Params.IdempotencyKey,
		RequestHash: hash[:],
	}
	if request.Body.Company != nil {
		company := model.Company{
			INN: request.Body.Company.Inn, Name: request.Body.Company.Name, OKVED: request.Body.Company.Okved,
			RegisteredAt:       request.Body.Company.RegisteredAt.Time,
			Revenue12mKopecks:  request.Body.Company.Revenue12m.AmountKopecks,
			Profit12mKopecks:   request.Body.Company.Profit12m.AmountKopecks,
			ObligationsKopecks: request.Body.Company.Obligations.AmountKopecks,
		}
		params.Company = &company
	}
	if request.Body.Fields != nil {
		patch := model.ApplicationPatch{}
		if request.Body.Fields.Amount != nil {
			amount := request.Body.Fields.Amount.AmountKopecks
			patch.AmountKopecks = &amount
			patch.AmountCurrency = string(request.Body.Fields.Amount.Currency)
		}
		if request.Body.Fields.TermMonths != nil {
			term := *request.Body.Fields.TermMonths
			patch.TermMonths = &term
		}
		if request.Body.Fields.Purpose != nil {
			purpose := string(*request.Body.Fields.Purpose)
			patch.Purpose = &purpose
		}
		params.Fields = &patch
	}
	if request.Body.DocumentIds != nil {
		params.DocumentsSpecified = true
		params.DocumentIDs = append([]uuid.UUID(nil), (*request.Body.DocumentIds)...)
	}
	accepted, err := h.usecase.Execute(ctx, params)
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.ProvideInfo202JSONResponse{
		ApplicationId: common.Id(accepted.ApplicationID), PendingAction: accepted.PendingAction,
	}, nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitProvideInfoResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
