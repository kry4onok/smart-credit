package receive

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

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
				Inn: "7707083893", Name: "Test", Okved: "10.71", RegisteredAt: "2020-01-01",
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

func TestParseSubmittedEventAllowsMissingFinancialFields(t *testing.T) {
	event := submittedEvent()
	event.GetApplicationSubmitted().Company.Revenue_12M = nil
	event.GetApplicationSubmitted().Company.Profit_12M = nil
	event.GetApplicationSubmitted().Company.Obligations = nil
	input, err := parse(event)
	if err != nil {
		t.Fatal(err)
	}
	var company commonpb.CompanySnapshot
	if err := protojson.Unmarshal(input.CompanyJSON, &company); err != nil {
		t.Fatal(err)
	}
	if company.GetRevenue_12M() != nil || company.GetProfit_12M() != nil || company.GetObligations() != nil {
		t.Fatalf("missing fields became values: %v", &company)
	}
}

func TestParseSubmittedPinsProfileReferenceDate(t *testing.T) {
	event := submittedEvent()
	event.OccurredAt = timestamppb.New(time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC))
	version := int32(2)
	event.GetApplicationSubmitted().Company.ProfileSchemaVersion = &version
	submitted := event.GetApplicationSubmitted()
	submitted.RequirementsReferenceDate = "2026-10-01"
	input, err := parse(event)
	if err != nil {
		t.Fatal(err)
	}
	if input.RequirementsReferenceDate == nil || input.RequirementsReferenceDate.Format("2006-01-02") != "2026-10-01" {
		t.Fatalf("reference date = %v", input.RequirementsReferenceDate)
	}
	for _, value := range []string{"", "2026-10-27", "2026-02-30"} {
		submitted.RequirementsReferenceDate = value
		if _, err := parse(event); !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("date %q accepted: %v", value, err)
		}
	}
}

func TestParseV2DocumentAndAnsweredIDs(t *testing.T) {
	event := providedEvent()
	version := int32(2)
	event.GetInfoProvided().Company.ProfileSchemaVersion = &version
	doc := event.GetInfoProvided().Documents[0]
	doc.DocumentType = commonpb.DocumentType_DOCUMENT_TYPE_MANAGEMENT_REPORT
	doc.RequirementId = "business_result:2025-10-01:2026-09-30:all"
	doc.CoveragePeriod = &commonpb.DatePeriod{From: "2025-10-01", To: "2026-09-30"}
	event.GetInfoProvided().AnsweredRequirementIds = []string{"company.profit_12m"}
	input, err := parse(event)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.AnsweredRequirementIDs) != 1 || input.AnsweredRequirementIDs[0] != "company.profit_12m" {
		t.Fatalf("answered IDs: %v", input.AnsweredRequirementIDs)
	}
	var documents []DocumentSnapshot
	if err := json.Unmarshal(input.DocumentsJSON, &documents); err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0].DocumentType != "MANAGEMENT_REPORT" || documents[0].RequirementID != doc.RequirementId || documents[0].CoveragePeriod.GetFrom() != "2025-10-01" {
		t.Fatalf("document snapshot: %+v", documents)
	}
}

func TestParseInfoProvidedCarriesInvalidDocumentToManualReview(t *testing.T) {
	event := providedEvent()
	version := int32(2)
	event.GetInfoProvided().Company.ProfileSchemaVersion = &version
	doc := event.GetInfoProvided().Documents[0]
	doc.RequirementId = "cash_flow:2025-10-01:2026-09-30:all"
	doc.CoveragePeriod = &commonpb.DatePeriod{From: "2026-02-30", To: "2026-09-30"}
	input, err := parse(event)
	if err != nil || !input.InvalidDocumentMetadata {
		t.Fatalf("invalid v2 document: input=%+v, err=%v", input, err)
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
		{"missing okved", func(e *eventspb.Event) { e.GetApplicationSubmitted().Company.Okved = "" }},
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

func providedEvent() *eventspb.Event {
	event := submittedEvent()
	event.EventId = "11111111-1111-4111-8111-111111111111"
	event.AggregateVersion = 2
	submitted := event.GetApplicationSubmitted()
	event.Payload = &eventspb.Event_InfoProvided{InfoProvided: &eventspb.InfoProvided{
		RequestId: "22222222-2222-4222-8222-222222222222",
		Company:   submitted.Company, Form: submitted.Form, Documents: submitted.Documents,
	}}
	return event
}

func TestParseInfoProvidedPreservesFullSnapshots(t *testing.T) {
	event := providedEvent()
	event.GetInfoProvided().Company.Revenue_12M = nil
	input, err := parse(event)
	if err != nil {
		t.Fatal(err)
	}
	if !input.InfoProvided || input.RequestID.String() != event.GetInfoProvided().GetRequestId() ||
		input.AmountKopecks != 1000 || input.PurposeTag != "WORKING_CAPITAL" {
		t.Fatalf("InfoProvided input = %+v", input)
	}
	var company commonpb.CompanySnapshot
	if err := protojson.Unmarshal(input.CompanyJSON, &company); err != nil {
		t.Fatal(err)
	}
	if company.GetRevenue_12M() != nil {
		t.Fatal("missing revenue became a value")
	}
	var documents []map[string]any
	if err := json.Unmarshal(input.DocumentsJSON, &documents); err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents[0]["document_type"] != "BANK_STATEMENT" {
		t.Fatalf("documents = %v", documents)
	}
}

func TestParseRejectsInvalidInfoProvided(t *testing.T) {
	tests := []struct {
		name   string
		change func(*eventspb.Event)
	}{
		{"missing request ID", func(e *eventspb.Event) { e.GetInfoProvided().RequestId = "" }},
		{"missing company", func(e *eventspb.Event) { e.GetInfoProvided().Company = nil }},
		{"missing form", func(e *eventspb.Event) { e.GetInfoProvided().Form = nil }},
		{"invalid document", func(e *eventspb.Event) {
			e.GetInfoProvided().Documents[0].DocumentType = commonpb.DocumentType_DOCUMENT_TYPE_UNSPECIFIED
		}},
		{"stale version", func(e *eventspb.Event) { e.AggregateVersion = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := providedEvent()
			tt.change(event)
			if _, err := parse(event); !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("parse error = %v, want ErrInvalidEvent", err)
			}
		})
	}
}

func TestParseOfferResponses(t *testing.T) {
	for _, test := range []struct {
		name string
		make func(string) *eventspb.Event
	}{
		{"accepted", func(id string) *eventspb.Event {
			e := submittedEvent()
			e.Payload = &eventspb.Event_OfferAccepted{OfferAccepted: &eventspb.OfferAccepted{OfferId: id}}
			return e
		}},
		{"declined", func(id string) *eventspb.Event {
			e := submittedEvent()
			e.Payload = &eventspb.Event_OfferDeclined{OfferDeclined: &eventspb.OfferDeclined{OfferId: id}}
			return e
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := test.make("11111111-1111-4111-8111-111111111111")
			input, err := parse(event)
			if err != nil || input.OfferID.String() != "11111111-1111-4111-8111-111111111111" || input.OfferAction == "" {
				t.Fatalf("parsed response = %+v, %v", input, err)
			}
			if _, err := parse(test.make("")); !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("missing offer ID error = %v", err)
			}
		})
	}
}

func TestParseApplicationCancelled(t *testing.T) {
	event := submittedEvent()
	event.Payload = &eventspb.Event_ApplicationCancelled{ApplicationCancelled: &eventspb.ApplicationCancelled{
		Reason: "Финансирование больше не требуется",
	}}
	input, err := parse(event)
	if err != nil || !input.Cancelled || input.CancelReason != event.GetApplicationCancelled().GetReason() ||
		input.ApplicationID.String() != event.GetAggregateId() || input.AggregateVersion != event.GetAggregateVersion() {
		t.Fatalf("parsed cancellation = %+v, %v", input, err)
	}
	event.Payload = &eventspb.Event_ApplicationCancelled{}
	if _, err := parse(event); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("missing payload error = %v", err)
	}
	event.Payload = &eventspb.Event_ApplicationCancelled{ApplicationCancelled: &eventspb.ApplicationCancelled{
		Reason: strings.Repeat("a", 501),
	}}
	if _, err := parse(event); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("long reason error = %v", err)
	}
}

func TestParseContractSigned(t *testing.T) {
	makeEvent := func() *eventspb.Event {
		event := submittedEvent()
		event.AggregateVersion = 3
		event.Payload = &eventspb.Event_ContractSigned{ContractSigned: &eventspb.ContractSigned{
			OfferId:  "11111111-1111-4111-8111-111111111111",
			SignedAt: event.OccurredAt,
		}}
		return event
	}
	event := makeEvent()
	input, err := parse(event)
	if err != nil || input.OfferAction != "ContractSigned" ||
		input.OfferID.String() != event.GetContractSigned().GetOfferId() ||
		!input.SignedAt.Equal(event.OccurredAt.AsTime()) {
		t.Fatalf("contract signed input = %+v, %v", input, err)
	}
	for _, test := range []struct {
		name string
		edit func(*eventspb.Event)
	}{
		{"missing payload", func(e *eventspb.Event) { e.Payload = &eventspb.Event_ContractSigned{} }},
		{"invalid offer", func(e *eventspb.Event) { e.GetContractSigned().OfferId = "bad" }},
		{"missing signed_at", func(e *eventspb.Event) { e.GetContractSigned().SignedAt = nil }},
		{"inconsistent signed_at", func(e *eventspb.Event) {
			e.GetContractSigned().SignedAt = timestamppb.New(e.OccurredAt.AsTime().Add(6 * time.Minute))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := makeEvent()
			test.edit(event)
			if _, err := parse(event); !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("parse error = %v", err)
			}
		})
	}
}

func TestOtherPurposeDescription(t *testing.T) {
	for _, tc := range []struct {
		name, description string
		valid             bool
	}{
		{"described", "Ремонт помещения, оборудование и сырьё", true},
		{"blank", " \n\t", false},
		{"limit", strings.Repeat("я", 1000), true},
		{"too long", strings.Repeat("я", 1001), false},
		{"null byte", "a\x00b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := submittedEvent()
			form := event.GetApplicationSubmitted().Form
			form.Purpose = commonpb.PurposeTag_PURPOSE_TAG_OTHER
			form.PurposeDescription = tc.description
			input, err := parse(event)
			if !tc.valid {
				if !errors.Is(err, ErrInvalidEvent) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var restored commonpb.ApplicationForm
			if err = protojson.Unmarshal(input.FormJSON, &restored); err != nil {
				t.Fatal(err)
			}
			if input.PurposeTag != "OTHER" || restored.GetPurposeDescription() != tc.description {
				t.Fatalf("lost purpose description: %v", &restored)
			}
		})
	}
}
