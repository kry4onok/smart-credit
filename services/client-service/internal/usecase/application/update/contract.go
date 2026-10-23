package update

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	Update(context.Context, model.UpdateApplicationParams) (model.Application, error)
}

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, orgID, applicationID uuid.UUID, patch model.ApplicationPatch, expectedVersion int64) (model.Application, error) {
	if err := domain.ValidateApplicationPatch(patch); err != nil {
		return model.Application{}, err
	}
	return u.repository.Update(ctx, model.UpdateApplicationParams{
		OrgID: orgID, ApplicationID: applicationID, ExpectedVersion: expectedVersion, Patch: patch,
	})
}
