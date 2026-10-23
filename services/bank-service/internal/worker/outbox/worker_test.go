package outboxworker

import (
	"context"
	"errors"
	"testing"

	outboxrepository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/outbox"
	"github.com/google/uuid"
)

type recordingStore struct {
	event    outboxrepository.Event
	marked   bool
	released bool
}

func (s *recordingStore) ClaimNext(context.Context) (outboxrepository.Event, bool, error) {
	return s.event, true, nil
}

func (s *recordingStore) MarkPublished(_ context.Context, event outboxrepository.Event) (bool, error) {
	if event.ID != s.event.ID {
		return false, errors.New("wrong event marked")
	}
	s.marked = true
	return true, nil
}

func (s *recordingStore) Release(_ context.Context, event outboxrepository.Event) error {
	if event.ID != s.event.ID {
		return errors.New("wrong event released")
	}
	s.released = true
	return nil
}

type recordingPublisher struct {
	store *recordingStore
	event outboxrepository.Event
	err   error
}

func (p *recordingPublisher) Publish(_ context.Context, event outboxrepository.Event) error {
	if p.store.marked {
		return errors.New("event marked before Kafka acknowledgement")
	}
	p.event = event
	return p.err
}

func TestRunOnceMarksOnlyAfterPublishing(t *testing.T) {
	store := &recordingStore{event: outboxrepository.Event{
		ID: uuid.New(), Topic: "bank.events", AggregateID: uuid.NewString(), Payload: []byte("protobuf"),
	}}
	publisher := &recordingPublisher{store: store}
	processed, err := New(store, publisher).RunOnce(context.Background())
	if err != nil || !processed || !store.marked || store.released ||
		publisher.event.ID != store.event.ID ||
		publisher.event.AggregateID != store.event.AggregateID ||
		string(publisher.event.Payload) != "protobuf" {
		t.Fatalf("RunOnce = processed=%v, err=%v, marked=%v, released=%v, published=%+v",
			processed, err, store.marked, store.released, publisher.event)
	}
}

func TestRunOnceReleasesAfterPublishFailure(t *testing.T) {
	store := &recordingStore{event: outboxrepository.Event{ID: uuid.New()}}
	publisher := &recordingPublisher{store: store, err: errors.New("Kafka unavailable")}
	processed, err := New(store, publisher).RunOnce(context.Background())
	if !processed || err == nil || store.marked || !store.released {
		t.Fatalf("RunOnce = processed=%v, err=%v, marked=%v, released=%v",
			processed, err, store.marked, store.released)
	}
}
