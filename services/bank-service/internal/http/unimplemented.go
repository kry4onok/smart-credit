package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) SummarizeCase(context.Context, generated.SummarizeCaseRequestObject) (generated.SummarizeCaseResponseObject, error) {
	return nil, errNotImplemented
}
