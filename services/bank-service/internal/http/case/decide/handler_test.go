package decidehandler

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/decide"
	"github.com/google/uuid"
)

type useCaseStub struct {
	input decide.Input
	calls int
	err   error
}

func (s *useCaseStub) Execute(_ context.Context, in decide.Input) (model.Case, error) {
	s.calls++
	s.input = in
	return model.Case{ID: in.CaseID, Status: model.CaseStatusRejected, Version: in.ExpectedVersion + 1}, s.err
}
func request() generated.DecideCaseRequestObject {
	reason := "Не соответствует условиям"
	return generated.DecideCaseRequestObject{ApplicationId: uuid.New(), Params: generated.DecideCaseParams{IdempotencyKey: uuid.New(), IfMatch: `"7"`}, Body: &generated.AnalystDecisionRequest{Outcome: "REJECT", Comment: "Internal", PublicReason: &reason}}
}
func TestDecisionRequiresAnalyst(t *testing.T) {
	stub := &useCaseStub{}
	_, err := New(stub).Handle(t.Context(), request())
	var de *domainerror.Error
	if !errors.As(err, &de) || de.Status != 403 || stub.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, stub.calls)
	}
}
func TestDecisionPassesVersionAndMapsConflict(t *testing.T) {
	stub := &useCaseStub{err: decide.ErrConflict}
	analyst := uuid.New()
	req := request()
	_, err := New(stub).Handle(authmiddleware.WithAnalystID(t.Context(), analyst), req)
	var de *domainerror.Error
	if !errors.As(err, &de) || de.Status != 409 || stub.input.ExpectedVersion != 7 || stub.input.AnalystID != analyst || stub.input.PublicReason != *req.Body.PublicReason {
		t.Fatalf("error=%v input=%+v", err, stub.input)
	}
}
func TestDecisionBadETagNeverCallsUseCase(t *testing.T) {
	stub := &useCaseStub{}
	req := request()
	req.Params.IfMatch = "7"
	_, err := New(stub).Handle(authmiddleware.WithAnalystID(t.Context(), uuid.New()), req)
	var de *domainerror.Error
	if !errors.As(err, &de) || de.Status != 400 || stub.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, stub.calls)
	}
}

func TestV2RequirementIDsAndRuleErrorMapping(t *testing.T) {
	ids := []string{"document:2025-01-01:2025-12-31:all"}
	stub := &useCaseStub{err: &requirements.RuleError{Code: requirements.UnknownRequirement}}
	req := request()
	req.Body.Outcome = "REQUEST_INFO"
	req.Body.InfoRequest = &generated.InfoRequestDraft{Message: "Добавьте документ", RequirementIds: &ids}
	_, err := New(stub).Handle(authmiddleware.WithAnalystID(t.Context(), uuid.New()), req)
	var de *domainerror.Error
	if !errors.As(err, &de) || de.Status != 422 || de.Code != requirements.UnknownRequirement ||
		stub.input.InfoRequest == nil || !stub.input.InfoRequest.V2 || len(stub.input.InfoRequest.RequirementIDs) != 1 {
		t.Fatalf("error=%v input=%+v", err, stub.input)
	}
}
