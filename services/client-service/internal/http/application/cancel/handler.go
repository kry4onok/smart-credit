package cancel

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

func (h *Handler) Handle(ctx context.Context, request generated.CancelApplicationRequestObject) (generated.CancelApplicationResponseObject, error) {
	client, ok := middleware.Client(ctx)
	if !ok {
		return problemResponse{domainerror.Problem(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия.")}, nil
	}
	if request.ApplicationId == uuid.Nil || request.Params.IdempotencyKey == uuid.Nil {
		return problemResponse{domainerror.Problem(ctx, http.StatusBadRequest, "INVALID_CANCEL_ACTION", "ID заявки и Idempotency-Key должны быть ненулевыми UUID.")}, nil
	}
	body := generated.CancelApplicationJSONRequestBody{}
	if request.Body != nil {
		body = *request.Body
	}
	hashInput, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(hashInput)
	var reason string
	if body.Reason != nil {
		reason = *body.Reason
	}
	accepted, err := h.usecase.Execute(ctx, model.CancelApplicationParams{
		OrgID: client.OrgID, UserID: client.UserID, ApplicationID: request.ApplicationId,
		IdempotencyID: request.Params.IdempotencyKey, Reason: reason, RequestHash: hash[:],
	})
	if err != nil {
		return problemResponse{domainerror.Map(ctx, err)}, nil
	}
	return generated.CancelApplication202JSONResponse{
		ApplicationId: common.Id(accepted.ApplicationID), PendingAction: accepted.PendingAction,
	}, nil
}

type problemResponse struct{ common.Problem }

func (r problemResponse) VisitCancelApplicationResponse(w http.ResponseWriter) error {
	domainerror.Write(w, r.Problem)
	return nil
}
