package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) TakeCase(ctx context.Context, request generated.TakeCaseRequestObject) (generated.TakeCaseResponseObject, error) {
	return s.takeCase.Handle(ctx, request)
}
