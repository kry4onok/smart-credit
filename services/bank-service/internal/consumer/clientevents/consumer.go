package clientevents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

const retryDelay = 2 * time.Second
const topic = "client.events"

type Receiver interface {
	Execute(context.Context, *eventspb.Event) (bool, error)
}

type Consumer struct {
	client   *kgo.Client
	receiver Receiver
}

func New(brokers []string, group string, receiver Receiver) (*Consumer, error) {
	if len(brokers) == 0 || group == "" || receiver == nil {
		return nil, errors.New("kafka brokers, consumer group and receiver are required")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, fmt.Errorf("create client events consumer: %w", err)
	}
	return &Consumer{client: client, receiver: receiver}, nil
}

func (c *Consumer) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

func (c *Consumer) Run(ctx context.Context, logger *slog.Logger) {
	for ctx.Err() == nil {
		fetches := c.client.PollRecords(ctx, 1)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return
		}
		fetchError := false
		fetches.EachError(func(topic string, partition int32, err error) {
			fetchError = true
			logger.Error("client events fetch failed", "topic", topic, "partition", partition, "error", err)
		})
		if fetchError && !pause(ctx) {
			return
		}
		var record *kgo.Record
		fetches.EachRecord(func(r *kgo.Record) { record = r })
		if record == nil {
			continue
		}
		for ctx.Err() == nil {
			eventID, applicationID, err := c.apply(ctx, record)
			if err == nil {
				err = c.client.CommitRecords(ctx, record)
			}
			if err == nil {
				break
			}
			logger.Error("client event processing failed", "topic", record.Topic,
				"partition", record.Partition, "offset", record.Offset,
				"event_id", eventID, "application_id", applicationID, "error", err)
			if !pause(ctx) {
				return
			}
		}
	}
}

func (c *Consumer) apply(ctx context.Context, record *kgo.Record) (string, string, error) {
	if record.Topic != topic {
		return "", "", fmt.Errorf("unexpected Kafka topic %q", record.Topic)
	}
	event := new(eventspb.Event)
	if err := proto.Unmarshal(record.Value, event); err != nil {
		return "", "", fmt.Errorf("decode protobuf event: %w", err)
	}
	if string(record.Key) != event.GetAggregateId() {
		return event.GetEventId(), event.GetAggregateId(), errors.New("kafka key differs from aggregate_id")
	}
	_, err := c.receiver.Execute(ctx, event)
	return event.GetEventId(), event.GetAggregateId(), err
}

func pause(ctx context.Context) bool {
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
