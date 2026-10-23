package submit

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type UseCase interface {
	Execute(context.Context, model.SubmitApplicationParams) (model.Application, error)
}

type Handler struct{ usecase UseCase }

func New(usecase UseCase) *Handler { return &Handler{usecase: usecase} }
