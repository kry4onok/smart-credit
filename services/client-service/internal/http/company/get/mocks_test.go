package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type usecaseMock struct {
	execute func(context.Context, uuid.UUID) (model.Company, error)
}

func (m usecaseMock) Execute(ctx context.Context, id uuid.UUID) (model.Company, error) {
	return m.execute(ctx, id)
}

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}
