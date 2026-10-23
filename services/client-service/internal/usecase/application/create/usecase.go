package create

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, orgID, userID, productID, idempotencyID uuid.UUID) (model.Application, error) {
	return u.repository.Create(ctx, model.CreateApplicationParams{
		OrgID: orgID, UserID: userID, ProductID: productID, IdempotencyID: idempotencyID,
	})
}
