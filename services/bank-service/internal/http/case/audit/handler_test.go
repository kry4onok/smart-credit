package audithandler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	auditcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/audit"
	"github.com/google/uuid"
)

type useCaseStub struct {
	calls  int
	input  auditcase.Input
	output auditcase.Output
	err    error
}

func (s *useCaseStub) Execute(_ context.Context, in auditcase.Input) (auditcase.Output, error) {
	s.calls++
	s.input = in
	return s.output, s.err
}
func TestAuditRequiresEmployee(t *testing.T) {
	s := &useCaseStub{}
	_, err := New(s).Handle(t.Context(), generated.GetCaseAuditRequestObject{ApplicationId: uuid.New()})
	var de *domainerror.Error
	if !errors.As(err, &de) || de.Status != 403 || s.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, s.calls)
	}
}
func TestAuditMapsEntriesAndLeavesUnknownStatusesOut(t *testing.T) {
	from, to := "MANUAL_REVIEW", "UNKNOWN"
	actor := uuid.New()
	at := time.Now().UTC()
	s := &useCaseStub{output: auditcase.Output{Items: []auditcase.Entry{{ID: uuid.New(), Action: "DECISION", ActorType: "ANALYST", ActorID: &actor, FromStatus: &from, ToStatus: &to, Reason: "APPROVE", OccurredAt: at}}}}
	ctx := authmiddleware.WithUserID(t.Context(), actor)
	caseID := uuid.New()
	response, err := New(s).Handle(ctx, generated.GetCaseAuditRequestObject{ApplicationId: caseID})
	if err != nil {
		t.Fatal(err)
	}
	page, ok := response.(generated.GetCaseAudit200JSONResponse)
	if !ok || len(page.Items) != 1 || page.Items[0].FromStatus == nil || page.Items[0].ToStatus != nil || page.Items[0].ActorId == nil || s.input.CaseID != caseID {
		t.Fatalf("page=%+v input=%+v", page, s.input)
	}
}
