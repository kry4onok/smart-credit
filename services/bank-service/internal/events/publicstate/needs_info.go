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

func NeedsInfo(eventID, caseID, requestID uuid.UUID, version int64, occurredAt time.Time,
	message string, fields, documentTypes []string,
) ([]byte, error) {
	request := &commonpb.InfoRequest{
		RequestId: requestID.String(), Message: message,
		CreatedAt: timestamppb.New(occurredAt),
	}
	for _, field := range fields {
		value, ok := commonpb.InfoField_value["INFO_FIELD_"+field]
		if !ok || value == 0 {
			return nil, fmt.Errorf("unknown info field %q", field)
		}
		request.RequiredFields = append(request.RequiredFields, commonpb.InfoField(value))
	}
	for _, documentType := range documentTypes {
		value, ok := commonpb.DocumentType_value["DOCUMENT_TYPE_"+documentType]
		if !ok || value == 0 {
			return nil, fmt.Errorf("unknown document type %q", documentType)
		}
		request.RequiredDocumentTypes = append(request.RequiredDocumentTypes, commonpb.DocumentType(value))
	}
	event := &eventspb.Event{
		EventId: eventID.String(), AggregateType: "APPLICATION", AggregateId: caseID.String(),
		AggregateVersion: version, OccurredAt: timestamppb.New(occurredAt),
		CorrelationId: caseID.String(),
		Payload: &eventspb.Event_ApplicationPublicStatePublished{ApplicationPublicStatePublished: &eventspb.ApplicationPublicStatePublished{
			Status:     eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO,
			StageLabel: "Нужны данные", NextAction: "Дополните заявку",
			ActiveInfoRequest: request, PublicReason: message,
		}},
	}
	return proto.Marshal(event)
}
