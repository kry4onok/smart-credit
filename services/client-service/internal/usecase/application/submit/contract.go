package submit

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	Submit(context.Context, model.SubmitApplicationParams) (model.Application, error)
}

type UseCase struct{ repository Repository }

func New(repository Repository) *UseCase { return &UseCase{repository: repository} }

func (u *UseCase) Execute(ctx context.Context, params model.SubmitApplicationParams) (model.Application, error) {
	if params.OrgID == uuid.Nil || params.UserID == uuid.Nil ||
		params.ApplicationID == uuid.Nil || params.IdempotencyID == uuid.Nil {
		return model.Application{}, domain.ErrInvalidApplication
	}
	return u.repository.Submit(ctx, params)
}
