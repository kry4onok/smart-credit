package listhandler

import (
	"context"

	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
)

type useCaseMock struct {
	execute func(context.Context, listcase.Input) (listcase.Output, error)
}

func (m useCaseMock) Execute(
	ctx context.Context,
	input listcase.Input,
) (listcase.Output, error) {
	return m.execute(ctx, input)
}
