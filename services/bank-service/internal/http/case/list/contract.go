package listhandler

import (
	"context"

	listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"
)

type UseCase interface {
	Execute(context.Context, listcase.Input) (listcase.Output, error)
}
