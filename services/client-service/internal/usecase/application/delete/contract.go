package delete

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type Repository interface {
	Delete(context.Context, model.DeleteApplicationParams) error
}
type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }
func (u *Usecase) Execute(ctx context.Context, params model.DeleteApplicationParams) error {
	return u.repository.Delete(ctx, params)
}
