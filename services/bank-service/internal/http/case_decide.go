package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) DecideCase(ctx context.Context, request generated.DecideCaseRequestObject) (generated.DecideCaseResponseObject, error) {
	return s.decideCase.Handle(ctx, request)
}
