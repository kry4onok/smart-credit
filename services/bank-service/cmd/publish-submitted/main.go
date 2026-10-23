package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	brokers := flag.String("brokers", "127.0.0.1:9092", "comma-separated Kafka broker addresses")
	applicationID := flag.String("application-id", uuid.NewString(), "application UUID (reuse to test a duplicate)")
	eventID := flag.String("event-id", uuid.NewString(), "event UUID (reuse to test a duplicate)")
	missingData := flag.Bool("missing-data", false, "omit revenue and documents to exercise NEEDS_INFO")
	flag.Parse()
	if err := run(*brokers, *applicationID, *eventID, *missingData); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(brokers, applicationID, eventID string, missingData bool) error {
	if _, err := uuid.Parse(applicationID); err != nil {
		return fmt.Errorf("invalid application ID: %w", err)
	}
	if _, err := uuid.Parse(eventID); err != nil {
		return fmt.Errorf("invalid event ID: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(brokers, ",")...))
	if err != nil {
		return fmt.Errorf("create Kafka producer: %w", err)
	}
	defer client.Close()
	content := &eventspb.Event{
		EventId: eventID, AggregateType: "APPLICATION", AggregateId: applicationID,
		AggregateVersion: 1, OccurredAt: timestamppb.Now(), CorrelationId: uuid.NewString(),
		Payload: &eventspb.Event_ApplicationSubmitted{ApplicationSubmitted: &eventspb.ApplicationSubmitted{
			OrgId: "20000000-0000-4000-8000-000000000001", ClientUserId: "30000000-0000-4000-8000-000000000001",
			ProductId: "11111111-1111-4111-8111-111111111111", ProductVersion: 1,
			Company: &commonpb.CompanySnapshot{
				Inn: "7707083893", Name: "ООО «Тестовая пекарня»", Okved: "10.71",
				RegisteredAt: "2019-04-15",
				Revenue_12M:  &commonpb.Money{AmountKopecks: 840000000, Currency: "RUB"},
				Profit_12M:   &commonpb.Money{AmountKopecks: 96000000, Currency: "RUB"},
				Obligations:  &commonpb.Money{AmountKopecks: 120000000, Currency: "RUB"},
			},
			Form: &commonpb.ApplicationForm{
				Amount:     &commonpb.Money{AmountKopecks: 300000000, Currency: "RUB"},
				TermMonths: 18, Purpose: commonpb.PurposeTag_PURPOSE_TAG_WORKING_CAPITAL,
			},
			Documents: []*commonpb.DocumentMetadata{{
				DocumentId: uuid.NewString(), DocumentType: commonpb.DocumentType_DOCUMENT_TYPE_BANK_STATEMENT,
				FileName: "demo-statement.pdf", Sha256: strings.Repeat("a", 64),
				UploadedAt: timestamppb.Now(), IsMock: true,
			}},
		}},
	}
	if missingData {
		content.GetApplicationSubmitted().Company.Revenue_12M = nil
		content.GetApplicationSubmitted().Documents = nil
	}
	value, err := proto.Marshal(content)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	if err := client.ProduceSync(ctx, &kgo.Record{
		Topic: "client.events", Key: []byte(applicationID), Value: value,
	}).FirstErr(); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	fmt.Printf("application_id=%s event_id=%s\n", applicationID, eventID)
	return nil
}
