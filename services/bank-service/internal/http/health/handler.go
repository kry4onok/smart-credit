package health

import (
	"context"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

const readinessTimeout = time.Second

type Pinger interface {
	Ping(context.Context) error
}

type Handler struct {
	db Pinger
}

func New(db Pinger) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Health(context.Context, generated.BankHealthRequestObject) (generated.BankHealthResponseObject, error) {
	return generated.BankHealth204Response{}, nil
}

func (h *Handler) Ready(ctx context.Context, _ generated.BankReadyRequestObject) (generated.BankReadyResponseObject, error) {
	if h.db == nil {
		return generated.BankReady503Response{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		return generated.BankReady503Response{}, nil
	}

	return generated.BankReady204Response{}, nil
}
