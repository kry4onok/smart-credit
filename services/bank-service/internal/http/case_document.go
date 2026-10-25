package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) GetCaseDocumentContent(ctx context.Context, request generated.GetCaseDocumentContentRequestObject) (generated.GetCaseDocumentContentResponseObject, error) {
	return s.getCaseDocument.Handle(ctx, request)
}
