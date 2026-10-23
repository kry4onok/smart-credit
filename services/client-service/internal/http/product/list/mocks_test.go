package list

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type usecaseMock struct {
	catalog model.ProductCatalog
	purpose *string
	err     error
}

func (m *usecaseMock) Execute(_ context.Context, purpose *string) (model.ProductCatalog, error) {
	m.purpose = purpose
	return m.catalog, m.err
}
