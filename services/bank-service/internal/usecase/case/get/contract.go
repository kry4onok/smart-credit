package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	GetCase(context.Context, uuid.UUID) (model.Case, bool, error)
}

type Input struct {
	CaseID        uuid.UUID
	CurrentUserID uuid.UUID
}

type Output struct {
	Case           model.Case
	AllowedActions []model.CaseAction
}
