package list

import (
	"context"

	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
)

type Usecase interface {
	Execute(context.Context, listapplication.Input) (listapplication.Output, error)
}

type Handler struct{ usecase Usecase }

func New(usecase Usecase) *Handler { return &Handler{usecase: usecase} }
