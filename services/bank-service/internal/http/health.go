package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) BankHealth(ctx context.Context, request generated.BankHealthRequestObject) (generated.BankHealthResponseObject, error) {
	return s.health.Health(ctx, request)
}

func (s *server) BankReady(ctx context.Context, request generated.BankReadyRequestObject) (generated.BankReadyResponseObject, error) {
	return s.health.Ready(ctx, request)
}
