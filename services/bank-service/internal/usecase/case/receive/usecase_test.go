package receive

import (
	"encoding/json"
	"errors"
	"testing"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func submittedEvent() *eventspb.Event {
	return &eventspb.Event{
		EventId:       "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		AggregateType: "APPLICATION", AggregateId: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		AggregateVersion: 1, OccurredAt: timestamppb.Now(), CorrelationId: "test",
		Payload: &eventspb.Event_ApplicationSubmitted{ApplicationSubmitted: &eventspb.ApplicationSubmitted{
			OrgId:        "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
			ClientUserId: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
			ProductId:    "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", ProductVersion: 2,
			Company: &commonpb.CompanySnapshot{
				Inn: "7707083893", Name: "Test", RegisteredAt: "2020-01-01",
				Revenue_12M: &commonpb.Money{AmountKopecks: 100, Currency: "RUB"},
				Profit_12M:  &commonpb.Money{AmountKopecks: -10, Currency: "RUB"},
				Obligations: &commonpb.Money{AmountKopecks: 50, Currency: "RUB"},
			},
			Form: &commonpb.ApplicationForm{
				Amount:     &commonpb.Money{AmountKopecks: 1000, Currency: "RUB"},
				TermMonths: 12, Purpose: commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL,
			},
			Documents: []*commonpb.DocumentMetadata{{
				DocumentId:   "ffffffff-ffff-4fff-8fff-ffffffffffff",
				DocumentType: commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT,
				UploadedAt:   timestamppb.Now(),
			}},
		}},
	}
}

func TestParseSubmittedEventPreservesSnapshots(t *testing.T) {
	input, err := parse(submittedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if input.AmountKopecks != 1000 || input.TermMonths != 12 || input.PurposeTag != "WORKING_CAPITAL" {
		t.Fatalf("form columns: %+v", input)
	}
	var company commonpb.CompanySnapshot
	if err := protojson.Unmarshal(input.CompanyJSON, &company); err != nil {
		t.Fatal(err)
	}
	if company.GetProfit_12M().GetAmountKopecks() != -10 {
		t.Fatalf("lost signed profit: %v", company.GetProfit_12M())
	}
	var documents []map[string]any
	if err := json.Unmarshal(input.DocumentsJSON, &documents); err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0]["document_type"] != "BANK_STATEMENT" {
		t.Fatalf("document snapshot: %v", documents)
	}
}

func TestParseRejectsInvalidSubmittedEvent(t *testing.T) {
	tests := []struct {
		name   string
		change func(*eventspb.Event)
	}{
		{"missing payload", func(e *eventspb.Event) { e.Payload = nil }},
		{"wrong aggregate", func(e *eventspb.Event) { e.AggregateType = "CATALOG" }},
		{"invalid event ID", func(e *eventspb.Event) { e.EventId = "bad" }},
		{"invalid purpose", func(e *eventspb.Event) {
			e.GetApplicationSubmitted().Form.Purpose = commonpb.PurposeTag_PURPOSE_TAG_UNSPECIFIED
		}},
		{"missing amount", func(e *eventspb.Event) { e.GetApplicationSubmitted().Form.Amount = nil }},
		{"invalid date", func(e *eventspb.Event) { e.GetApplicationSubmitted().Company.RegisteredAt = "2020-02-30" }},
		{"invalid document", func(e *eventspb.Event) {
			e.GetApplicationSubmitted().Documents[0].DocumentType = commonpb.DocumentType_DOCUMENT_TYPE_UNSPECIFIED
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := submittedEvent()
			tt.change(event)
			_, err := parse(event)
			if !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("parse error = %v, want ErrInvalidEvent", err)
			}
		})
	}
}
