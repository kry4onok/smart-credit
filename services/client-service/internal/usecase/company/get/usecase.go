package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, orgID uuid.UUID) (model.Company, error) {
	return u.repository.Fetch(ctx, orgID)
}
