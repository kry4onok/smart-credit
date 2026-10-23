package list

import (
	"context"

	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
)

type usecaseMock struct {
	output listapplication.Output
	err    error
	input  listapplication.Input
}

func (m *usecaseMock) Execute(_ context.Context, input listapplication.Input) (listapplication.Output, error) {
	m.input = input
	return m.output, m.err
}
