package takehandler

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/take"
	"github.com/google/uuid"
)

type useCaseStub struct {
	input  take.Input
	result model.Case
	err    error
	calls  int
}

func (s *useCaseStub) Execute(_ context.Context, input take.Input) (model.Case, error) {
	s.calls++
	s.input = input
	return s.result, s.err
}

func TestTakeRequiresVerifiedAnalyst(t *testing.T) {
	stub := &useCaseStub{}
	_, err := New(stub).Handle(t.Context(), generated.TakeCaseRequestObject{})
	var domainErr *domainerror.Error
	if !errors.As(err, &domainErr) || domainErr.Status != 403 || stub.calls != 0 {
		t.Fatalf("error = %v, calls = %d; want 403 and no call", err, stub.calls)
	}
}

func TestTakeReturnsCaseAndNewETag(t *testing.T) {
	analystID, caseID, key := uuid.New(), uuid.New(), uuid.New()
	stub := &useCaseStub{result: model.Case{
		ID: caseID, Status: model.CaseStatusManualReview,
		AssigneeID: &analystID, Version: 8,
	}}
	ctx := authmiddleware.WithAnalystID(t.Context(), analystID)
	response, err := New(stub).Handle(ctx, generated.TakeCaseRequestObject{
		ApplicationId: caseID,
		Params:        generated.TakeCaseParams{IdempotencyKey: key, IfMatch: `"7"`},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := response.(generated.TakeCase200JSONResponse)
	if !ok || result.Headers.ETag == nil || *result.Headers.ETag != `"8"` || result.Body.Version != 8 {
		t.Fatalf("response = %#v; want case at version 8 and ETag", response)
	}
	if stub.input.AnalystID != analystID || stub.input.ExpectedVersion != 7 || stub.input.IdempotencyKey != key {
		t.Fatalf("input = %#v", stub.input)
	}
}

func TestTakeRejectsStaleVersion(t *testing.T) {
	stub := &useCaseStub{err: take.ErrConflict}
	ctx := authmiddleware.WithAnalystID(t.Context(), uuid.New())
	_, err := New(stub).Handle(ctx, generated.TakeCaseRequestObject{
		ApplicationId: uuid.New(),
		Params:        generated.TakeCaseParams{IdempotencyKey: uuid.New(), IfMatch: `"3"`},
	})
	var domainErr *domainerror.Error
	if !errors.As(err, &domainErr) || domainErr.Status != 409 {
		t.Fatalf("error = %v; want 409", err)
	}
}
