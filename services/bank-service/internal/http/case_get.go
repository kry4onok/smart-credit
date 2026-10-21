package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) GetCase(
	ctx context.Context,
	request generated.GetCaseRequestObject,
) (generated.GetCaseResponseObject, error) {
	return s.getCase.Handle(ctx, request)
}
