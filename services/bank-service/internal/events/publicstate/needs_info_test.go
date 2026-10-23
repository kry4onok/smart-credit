package publicstate

import (
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func TestNeedsInfoContainsFullPublicRequest(t *testing.T) {
	eventID, caseID, requestID := uuid.New(), uuid.New(), uuid.New()
	payload, err := NeedsInfo(eventID, caseID, requestID, 2, time.Now().UTC(), "Дополните заявку",
		[]string{"COMPANY_REVENUE_12M"}, []string{"BANK_STATEMENT"})
	if err != nil {
		t.Fatal(err)
	}
	var event eventspb.Event
	if err := proto.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	state := event.GetApplicationPublicStatePublished()
	if event.GetEventId() != eventID.String() || event.GetAggregateId() != caseID.String() ||
		event.GetAggregateVersion() != 2 || state.GetStatus() != eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO {
		t.Fatalf("event = %v", &event)
	}
	request := state.GetActiveInfoRequest()
	if request.GetRequestId() != requestID.String() ||
		len(request.GetRequiredFields()) != 1 ||
		request.GetRequiredFields()[0] != commonpb.InfoField_INFO_FIELD_COMPANY_REVENUE_12M ||
		len(request.GetRequiredDocumentTypes()) != 1 ||
		request.GetRequiredDocumentTypes()[0] != commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT {
		t.Fatalf("info request = %v", request)
	}
}
