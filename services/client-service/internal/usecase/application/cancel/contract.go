package cancel

import (
	"context"
	"unicode/utf8"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	CancelApplication(context.Context, model.CancelApplicationParams) (model.ActionAccepted, error)
}

type UseCase struct{ repository Repository }

func New(repository Repository) *UseCase { return &UseCase{repository: repository} }

func (u *UseCase) Execute(ctx context.Context, params model.CancelApplicationParams) (model.ActionAccepted, error) {
	if params.OrgID == uuid.Nil || params.UserID == uuid.Nil || params.ApplicationID == uuid.Nil ||
		params.IdempotencyID == uuid.Nil || len(params.RequestHash) != 32 ||
		!utf8.ValidString(params.Reason) || utf8.RuneCountInString(params.Reason) > 500 {
		return model.ActionAccepted{}, domain.ErrInvalidCancelAction
	}
	return u.repository.CancelApplication(ctx, params)
}
