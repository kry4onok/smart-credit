package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

type repositoryMock struct {
	getCase func(context.Context, uuid.UUID) (model.Case, bool, error)
}

func (m repositoryMock) GetCase(
	ctx context.Context,
	caseID uuid.UUID,
) (model.Case, bool, error) {
	return m.getCase(ctx, caseID)
}
