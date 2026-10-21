package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) ListCases(
	ctx context.Context,
	request generated.ListCasesRequestObject,
) (generated.ListCasesResponseObject, error) {
	return s.listCases.Handle(ctx, request)
}
