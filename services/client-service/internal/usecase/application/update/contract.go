package update

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type Repository interface {
	Update(context.Context, model.UpdateApplicationParams) (model.Application, error)
}

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, params model.UpdateApplicationParams) (model.Application, error) {
	if err := domain.ValidateApplicationPatch(params.Patch); err != nil {
		return model.Application{}, err
	}
	return u.repository.Update(ctx, params)
}
