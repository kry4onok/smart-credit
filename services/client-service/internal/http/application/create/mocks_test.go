package create

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type usecaseMock struct {
	application model.Application
	err         error
	called      bool
}

func (m *usecaseMock) Execute(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) (model.Application, error) {
	m.called = true
	return m.application, m.err
}
