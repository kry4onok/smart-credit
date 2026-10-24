package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) GetCaseAudit(ctx context.Context, request generated.GetCaseAuditRequestObject) (generated.GetCaseAuditResponseObject, error) {
	return s.auditCase.Handle(ctx, request)
}
