package audithandler

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	auditcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/audit"
	"github.com/google/uuid"
)

type UseCase interface {
	Execute(context.Context, auditcase.Input) (auditcase.Output, error)
}
type Handler struct{ useCase UseCase }

func New(u UseCase) *Handler { return &Handler{u} }
func (h *Handler) Handle(ctx context.Context, request generated.GetCaseAuditRequestObject) (generated.GetCaseAuditResponseObject, error) {
	if _, ok := authmiddleware.UserID(ctx); !ok {
		return nil, domainerror.Forbidden("EMPLOYEE_REQUIRED", errors.New("требуется подтверждённый сотрудник банка"))
	}
	if request.ApplicationId == uuid.Nil {
		return nil, domainerror.BadRequest("INVALID_CASE_ID", errors.New("ID дела должен быть UUID"))
	}
	output, err := h.useCase.Execute(ctx, auditcase.Input{CaseID: request.ApplicationId, Cursor: request.Params.Cursor, Limit: request.Params.Limit})
	if err != nil {
		switch {
		case errors.Is(err, auditcase.ErrNotFound):
			return nil, domainerror.NotFound("CASE_NOT_FOUND", err)
		case errors.Is(err, auditcase.ErrInvalidCursor), errors.Is(err, auditcase.ErrInvalidLimit):
			return nil, domainerror.BadRequest("INVALID_AUDIT_PAGE", err)
		default:
			return nil, fmt.Errorf("execute case audit: %w", err)
		}
	}
	items := make([]generated.AuditEntry, 0, len(output.Items))
	for _, item := range output.Items {
		items = append(items, generated.AuditEntry{Id: item.ID, Action: item.Action, ActorType: generated.AuditEntryActorType(item.ActorType), ActorId: item.ActorID, FromStatus: bankStatus(item.FromStatus), ToStatus: bankStatus(item.ToStatus), Reason: item.Reason, OccurredAt: item.OccurredAt})
	}
	return generated.GetCaseAudit200JSONResponse{Items: items, NextCursor: output.NextCursor}, nil
}
func bankStatus(value *string) *common.BankStatus {
	if value == nil {
		return nil
	}
	status := common.BankStatus(*value)
	if !status.Valid() {
		return nil
	}
	return &status
}
