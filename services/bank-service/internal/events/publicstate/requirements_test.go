package publicstate

import (
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func TestWithRequirementsPublishesPlanAndActiveRequest(t *testing.T) {
	eventID, caseID, requestID := uuid.New(), uuid.New(), uuid.New()
	payload, err := NeedsInfo(eventID, caseID, requestID, 3, time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), "Нужен файл", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	item := &commonpb.Requirement{Id: "cash_flow:2025-10-01:2026-09-30:all", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT, Status: commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING}
	plan := &commonpb.RequirementsPlan{CanSubmit: true, Items: []*commonpb.Requirement{item}}
	payload, err = WithRequirements(payload, plan, []*commonpb.Requirement{item})
	if err != nil {
		t.Fatal(err)
	}
	decoded := new(eventspb.Event)
	if err := proto.Unmarshal(payload, decoded); err != nil {
		t.Fatal(err)
	}
	state := decoded.GetApplicationPublicStatePublished()
	if state.GetActiveInfoRequest().GetRequestId() != requestID.String() || len(state.GetActiveInfoRequest().GetRequirements()) != 1 || state.GetRequirementsPlan() == nil || state.GetRequirementsPlan().CanSubmit {
		t.Fatalf("public state: %v", state)
	}
	if !plan.CanSubmit {
		t.Fatal("input plan mutated")
	}
}
