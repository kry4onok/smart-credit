package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) SummarizeCase(ctx context.Context, request generated.SummarizeCaseRequestObject) (generated.SummarizeCaseResponseObject, error) {
	return s.summaryCase.Handle(ctx, request)
}
