package decline

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

func (h *Handler) Handle(ctx context.Context, request generated.DeclineOfferRequestObject) (generated.DeclineOfferResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	if request.ApplicationId == uuid.Nil || request.Params.IdempotencyKey == uuid.Nil || request.Body == nil || request.Body.OfferId == uuid.Nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_OFFER_ACTION", "ID заявки, offer_id и Idempotency-Key должны быть ненулевыми UUID.")}, nil
	}

	hashInput, err := json.Marshal(request.Body)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(hashInput)
	accepted, err := h.usecase.Execute(ctx, model.DeclineOfferParams{
		OrgID: client.OrgID, UserID: client.UserID, ApplicationID: request.ApplicationId,
		OfferID: request.Body.OfferId, IdempotencyID: request.Params.IdempotencyKey,
		RequestHash: hash[:],
	})
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.DeclineOffer202JSONResponse{
		ApplicationId: common.Id(accepted.ApplicationID), PendingAction: accepted.PendingAction,
	}, nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitDeclineOfferResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
