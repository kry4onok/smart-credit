package update

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	Create(context.Context, uuid.UUID, model.Company) (model.Company, error)
	Update(context.Context, uuid.UUID, model.Company, int64) (model.Company, error)
}

type Usecase interface {
	Execute(context.Context, uuid.UUID, model.Company, *int64) (model.Company, error)
}
