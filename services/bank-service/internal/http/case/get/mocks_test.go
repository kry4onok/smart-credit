package gethandler

import (
	"context"

	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
)

type useCaseMock struct {
	execute func(context.Context, getcase.Input) (getcase.Output, error)
}

func (m useCaseMock) Execute(
	ctx context.Context,
	input getcase.Input,
) (getcase.Output, error) {
	return m.execute(ctx, input)
}
