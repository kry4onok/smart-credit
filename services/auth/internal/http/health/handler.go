package health

import (
	"context"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
)

type Handler struct {
	db      Pinger
	timeout time.Duration
}

func New(db Pinger, timeout time.Duration) *Handler {
	return &Handler{db: db, timeout: timeout}
}

func (h *Handler) Health(context.Context, generated.AuthHealthRequestObject) (generated.AuthHealthResponseObject, error) {
	return generated.AuthHealth204Response{}, nil
}

func (h *Handler) Ready(ctx context.Context, _ generated.AuthReadyRequestObject) (generated.AuthReadyResponseObject, error) {
	if h.db == nil {
		return generated.AuthReady503Response{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		return generated.AuthReady503Response{}, nil
	}
	return generated.AuthReady204Response{}, nil
}
