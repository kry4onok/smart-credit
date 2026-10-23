package update

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type usecaseMock struct {
	application model.Application
	err         error
	called      bool
	version     int64
}

func (m *usecaseMock) Execute(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ model.ApplicationPatch, version int64) (model.Application, error) {
	m.called = true
	m.version = version
	return m.application, m.err
}
