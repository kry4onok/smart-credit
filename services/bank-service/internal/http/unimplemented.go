package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
)

func (s *server) GetCase(context.Context, generated.GetCaseRequestObject) (generated.GetCaseResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetCaseAudit(context.Context, generated.GetCaseAuditRequestObject) (generated.GetCaseAuditResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) DecideCase(context.Context, generated.DecideCaseRequestObject) (generated.DecideCaseResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) SummarizeCase(context.Context, generated.SummarizeCaseRequestObject) (generated.SummarizeCaseResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) TakeCase(context.Context, generated.TakeCaseRequestObject) (generated.TakeCaseResponseObject, error) {
	return nil, errNotImplemented
}
