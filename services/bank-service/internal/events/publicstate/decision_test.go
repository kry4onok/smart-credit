package publicstate

import (
	"strings"
	"testing"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func TestDecisionPublicSnapshots(t *testing.T) {
	cases := []struct {
		outcome string
		offer   *commonpb.Offer
		request *commonpb.InfoRequest
		want    eventspb.ClientStatus
	}{
		{"APPROVE", &commonpb.Offer{OfferId: uuid.NewString()}, nil, eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE},
		{"COUNTER", &commonpb.Offer{OfferId: uuid.NewString()}, nil, eventspb.ClientStatus_CLIENT_STATUS_OFFER_AVAILABLE},
		{"REQUEST_INFO", nil, &commonpb.InfoRequest{RequestId: uuid.NewString(), Message: "Дополните заявку"}, eventspb.ClientStatus_CLIENT_STATUS_NEEDS_INFO},
		{"REJECT", nil, nil, eventspb.ClientStatus_CLIENT_STATUS_REJECTED},
	}
	for _, tt := range cases {
		t.Run(tt.outcome, func(t *testing.T) {
			payload, err := Decision(uuid.New(), uuid.New(), 4, time.Now().UTC(), tt.outcome, "Публичная причина", tt.offer, tt.request)
			if err != nil {
				t.Fatal(err)
			}
			var event eventspb.Event
			if err := proto.Unmarshal(payload, &event); err != nil {
				t.Fatal(err)
			}
			state := event.GetApplicationPublicStatePublished()
			if state.GetStatus() != tt.want || event.GetAggregateVersion() != 4 {
				t.Fatalf("event = %v", &event)
			}
			if strings.Contains(string(payload), "Internal review") {
				t.Fatal("internal comment leaked")
			}
		})
	}
}
