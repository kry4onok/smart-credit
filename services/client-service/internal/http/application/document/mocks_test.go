package document

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type usecaseMock struct {
	document model.ApplicationDocument
	err      error
	params   model.AddApplicationDocumentParams
	called   bool
}

func (m *usecaseMock) Execute(_ context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
	m.called = true
	m.params = params
	return m.document, m.err
}
