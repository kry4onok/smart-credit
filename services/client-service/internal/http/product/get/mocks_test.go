package get

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type usecaseMock struct {
	product model.Product
	err     error
	id      uuid.UUID
}

func (m *usecaseMock) Execute(_ context.Context, id uuid.UUID) (model.Product, error) {
	m.id = id
	return m.product, m.err
}
