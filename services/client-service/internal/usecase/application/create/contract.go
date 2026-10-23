package create

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type Repository interface {
	Create(context.Context, model.CreateApplicationParams) (model.Application, error)
}
