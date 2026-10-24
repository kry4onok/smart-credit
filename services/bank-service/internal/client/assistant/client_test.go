package assistant

import (
	"context"
	"errors"
	"testing"
	"time"

	assistantpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/summary"
	"google.golang.org/grpc"
)

type rpcStub struct {
	request  *assistantpb.SummarizeCaseRequest
	deadline time.Time
	response *assistantpb.TextResponse
	err      error
}

func (s *rpcStub) SummarizeCase(ctx context.Context, request *assistantpb.SummarizeCaseRequest, _ ...grpc.CallOption) (*assistantpb.TextResponse, error) {
	s.request = request
	s.deadline, _ = ctx.Deadline()
	return s.response, s.err
}

func TestSummarizeMapsFactsAndSetsDeadline(t *testing.T) {
	rpc := &rpcStub{response: &assistantpb.TextResponse{Text: "Текст", ModelVersion: "qwen2.5:3b"}}
	facts := summary.Facts{ApplicationID: "case-id", ProductName: "Продукт", Purpose: "WORKING_CAPITAL", Grade: "B", ReasonCodes: []string{"NO_OVERDUE"}, TriggeredRuleCodes: []string{"AUTO_OFFER"}}
	before := time.Now()
	text, source, err := New(rpc).Summarize(context.Background(), facts)
	if err != nil || text != "Текст" || source != "assistant" {
		t.Fatalf("text=%q source=%q err=%v", text, source, err)
	}
	if rpc.deadline.IsZero() || rpc.deadline.Before(before) || rpc.deadline.After(before.Add(requestTimeout+time.Second)) {
		t.Fatalf("deadline = %v", rpc.deadline)
	}
	if rpc.request.GetApplicationId() != facts.ApplicationID || rpc.request.GetGrade() != "B" ||
		len(rpc.request.GetReasonCodes()) != 1 || len(rpc.request.GetTriggeredRuleCodes()) != 1 {
		t.Fatalf("request = %+v", rpc.request)
	}
}

func TestSummarizePreservesAssistantTemplateSource(t *testing.T) {
	rpc := &rpcStub{response: &assistantpb.TextResponse{Text: "Шаблон", ModelVersion: "template"}}
	_, source, err := New(rpc).Summarize(context.Background(), summary.Facts{})
	if err != nil || source != "template" {
		t.Fatalf("source=%q err=%v", source, err)
	}
	rpc.err = errors.New("unavailable")
	if _, _, err = New(rpc).Summarize(context.Background(), summary.Facts{}); err == nil {
		t.Fatal("expected RPC error")
	}
}
