package update

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type usecaseMock struct {
	application model.Application
	err         error
	called      bool
	params      model.UpdateApplicationParams
}

func (m *usecaseMock) Execute(_ context.Context, params model.UpdateApplicationParams) (model.Application, error) {
	m.called = true
	m.params = params
	return m.application, m.err
}
