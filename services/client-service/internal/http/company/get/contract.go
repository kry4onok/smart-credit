package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Usecase interface {
	Execute(context.Context, uuid.UUID) (model.Company, error)
}
