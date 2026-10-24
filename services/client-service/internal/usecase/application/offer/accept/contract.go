package accept

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	AcceptOffer(context.Context, model.AcceptOfferParams) (model.ActionAccepted, error)
}

type UseCase struct{ repository Repository }

func New(repository Repository) *UseCase { return &UseCase{repository: repository} }

func (u *UseCase) Execute(ctx context.Context, params model.AcceptOfferParams) (model.ActionAccepted, error) {
	if params.OrgID == uuid.Nil || params.UserID == uuid.Nil ||
		params.ApplicationID == uuid.Nil || params.OfferID == uuid.Nil ||
		params.IdempotencyID == uuid.Nil || len(params.RequestHash) != 32 {
		return model.ActionAccepted{}, domain.ErrInvalidOfferAction
	}
	return u.repository.AcceptOffer(ctx, params)
}
