package gethandler

import (
	"context"

	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
)

type UseCase interface {
	Execute(context.Context, getcase.Input) (getcase.Output, error)
}
