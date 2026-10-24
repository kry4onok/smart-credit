package publicstate

import (
	"fmt"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Decision(eventID, caseID uuid.UUID, version int64, at time.Time, outcome, publicReason string, offer *commonpb.Offer, request *commonpb.InfoRequest) ([]byte, error) {
	state := &eventspb.ApplicationPublicStatePublished{PublicReason: publicReason}
	switch outcome {
	case "APPROVE", "COUNTER":
		if offer == nil {
			return nil, fmt.Errorf("offer is required")
		}
		state.Status = eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE
		state.StageLabel = "Есть предложение банка"
		state.NextAction = "Примите или отклоните предложение"
		state.ActiveOffer = offer
	case "REQUEST_INFO":
		if request == nil {
			return nil, fmt.Errorf("info request is required")
		}
		state.Status = eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO
		state.StageLabel = "Нужны данные"
		state.NextAction = "Дополните заявку"
		state.ActiveInfoRequest = request
		state.PublicReason = request.Message
	case "REJECT":
		state.Status = eventspb.ClientStatus_CLIENT_STATUS_REJECTED
		state.StageLabel = "Решение банка"
		state.NextAction = ""
	default:
		return nil, fmt.Errorf("unknown outcome %q", outcome)
	}
	event := &eventspb.Event{EventId: eventID.String(), AggregateType: "APPLICATION", AggregateId: caseID.String(), AggregateVersion: version, OccurredAt: timestamppb.New(at), CorrelationId: caseID.String(), Payload: &eventspb.Event_ApplicationPublicStatePublished{ApplicationPublicStatePublished: state}}
	return proto.Marshal(event)
}
