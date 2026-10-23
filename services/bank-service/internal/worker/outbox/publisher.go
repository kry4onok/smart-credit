package outboxworker

import (
	"context"
	"errors"
	"fmt"

	outboxrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/outbox"
	"github.com/twmb/franz-go/pkg/kgo"
)

type KafkaPublisher struct {
	client *kgo.Client
}

func NewKafkaPublisher(brokers []string) (*KafkaPublisher, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are required for bank outbox")
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return nil, fmt.Errorf("create bank events producer: %w", err)
	}
	return &KafkaPublisher{client: client}, nil
}

func (p *KafkaPublisher) Publish(ctx context.Context, event outboxrepository.Event) error {
	err := p.client.ProduceSync(ctx, &kgo.Record{
		Topic: event.Topic, Key: []byte(event.AggregateID), Value: event.Payload,
	}).FirstErr()
	if err != nil {
		return fmt.Errorf("publish bank event: %w", err)
	}
	return nil
}

func (p *KafkaPublisher) Close() {
	if p != nil && p.client != nil {
		p.client.Close()
	}
}
