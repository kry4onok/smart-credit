package publicstate

import (
	"fmt"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func ManualReview(eventID, caseID uuid.UUID, version int64, at time.Time, reason string) ([]byte, error) {
	event := new(eventspb.Event)
	if err := proto.Unmarshal(UnderReview(eventID, caseID, caseID, version, at), event); err != nil {
		return nil, err
	}
	event.GetApplicationPublicStatePublished().PublicReason = reason
	return proto.Marshal(event)
}

func WithRequirements(payload []byte, plan *commonpb.RequirementsPlan, requested []*commonpb.Requirement) ([]byte, error) {
	event := new(eventspb.Event)
	if err := proto.Unmarshal(payload, event); err != nil {
		return nil, fmt.Errorf("decode public event: %w", err)
	}
	state := event.GetApplicationPublicStatePublished()
	if state == nil || state.GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO ||
		state.GetActiveInfoRequest() == nil || state.GetActiveInfoRequest().GetRequestId() == "" ||
		plan == nil || len(requested) == 0 {
		return nil, fmt.Errorf("NEEDS_INFO event, active request, plan and requirements are required")
	}
	copyPlan := proto.Clone(plan).(*commonpb.RequirementsPlan)
	copyPlan.CanSubmit = false
	state.RequirementsPlan = copyPlan
	request := state.ActiveInfoRequest
	request.RequiredFields = nil
	request.RequiredDocumentTypes = nil
	request.Requirements = make([]*commonpb.Requirement, 0, len(requested))
	for _, item := range requested {
		if item == nil {
			return nil, fmt.Errorf("nil requirement")
		}
		request.Requirements = append(request.Requirements, proto.Clone(item).(*commonpb.Requirement))
	}
	return proto.Marshal(event)
}
