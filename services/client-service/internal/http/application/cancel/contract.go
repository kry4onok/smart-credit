package cancel

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
)

type Usecase interface {
	Execute(context.Context, model.CancelApplicationParams) (model.ActionAccepted, error)
}

type Handler struct{ usecase Usecase }

func New(usecase Usecase) *Handler { return &Handler{usecase: usecase} }
