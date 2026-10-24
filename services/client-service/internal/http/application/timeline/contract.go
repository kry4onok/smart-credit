package timeline

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Usecase interface {
	Execute(context.Context, uuid.UUID, uuid.UUID) ([]model.TimelineEntry, error)
}

type Handler struct{ usecase Usecase }

func New(usecase Usecase) *Handler { return &Handler{usecase: usecase} }
