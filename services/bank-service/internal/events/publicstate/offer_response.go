package publicstate

import (
	"fmt"
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func OfferResponse(eventID, caseID uuid.UUID, version int64, at time.Time, action string) ([]byte, error) {
	state := &eventspb.ApplicationPublicStatePublished{}
	switch action {
	case "OfferAccepted":
		state.Status = eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING
		state.StageLabel = "Оформление кредита"
		state.NextAction = "Подпишите договор"
	case "OfferDeclined":
		state.Status = eventspb.ClientStatus_CLIENT_STATUS_CLOSED
		state.StageLabel = "Предложение отклонено"
	case "OfferExpired":
		state.Status = eventspb.ClientStatus_CLIENT_STATUS_CLOSED
		state.StageLabel = "Срок предложения истёк"
	default:
		return nil, fmt.Errorf("unsupported offer response %q", action)
	}
	event := &eventspb.Event{
		EventId: eventID.String(), AggregateType: "APPLICATION", AggregateId: caseID.String(),
		AggregateVersion: version, OccurredAt: timestamppb.New(at),
		CorrelationId: caseID.String(),
		Payload:       &eventspb.Event_ApplicationPublicStatePublished{ApplicationPublicStatePublished: state},
	}
	return proto.Marshal(event)
}

func Cancelled(eventID, caseID uuid.UUID, version int64, at time.Time) ([]byte, error) {
	event := &eventspb.Event{
		EventId: eventID.String(), AggregateType: "APPLICATION", AggregateId: caseID.String(),
		AggregateVersion: version, OccurredAt: timestamppb.New(at),
		CorrelationId: caseID.String(),
		Payload: &eventspb.Event_ApplicationPublicStatePublished{ApplicationPublicStatePublished: &eventspb.ApplicationPublicStatePublished{
			Status: eventspb.ClientStatus_CLIENT_STATUS_CLOSED, StageLabel: "Заявка отозвана",
		}},
	}
	return proto.Marshal(event)
}
