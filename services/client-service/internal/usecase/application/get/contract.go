package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	Get(context.Context, uuid.UUID, uuid.UUID) (model.Application, error)
}

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, orgID, applicationID uuid.UUID) (model.Application, error) {
	return u.repository.Get(ctx, orgID, applicationID)
}
