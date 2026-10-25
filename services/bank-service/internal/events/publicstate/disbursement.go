package publicstate

import (
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Signing(eventID, caseID uuid.UUID, version int64, at time.Time) ([]byte, error) {
	return signingState(eventID, caseID, version, at,
		eventspb.ClientStatus_CLIENT_STATUS_FORMALIZING,
		"Договор подписан", "Ожидайте выдачи")
}

func Disbursed(eventID, caseID uuid.UUID, version int64, at time.Time) ([]byte, error) {
	return signingState(eventID, caseID, version, at,
		eventspb.ClientStatus_CLIENT_STATUS_DISBURSED,
		"Деньги выданы (демо)", "")
}

func signingState(eventID, caseID uuid.UUID, version int64, at time.Time,
	status eventspb.ClientStatus, label, nextAction string,
) ([]byte, error) {
	event := &eventspb.Event{
		EventId: eventID.String(), AggregateType: "APPLICATION", AggregateId: caseID.String(),
		AggregateVersion: version, OccurredAt: timestamppb.New(at),
		CorrelationId: caseID.String(),
		Payload: &eventspb.Event_ApplicationPublicStatePublished{
			ApplicationPublicStatePublished: &eventspb.ApplicationPublicStatePublished{
				Status: status, StageLabel: label, NextAction: nextAction,
			},
		},
	}
	return proto.Marshal(event)
}
