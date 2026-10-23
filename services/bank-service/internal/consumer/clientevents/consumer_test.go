package clientevents

import (
	"context"
	"testing"

	eventspb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/events"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"
)

type recordingReceiver struct{ calls int }

func (r *recordingReceiver) Execute(context.Context, *eventspb.Event) (bool, error) {
	r.calls++
	return true, nil
}

func TestApplyChecksKafkaKeyBeforeReceiving(t *testing.T) {
	receiver := &recordingReceiver{}
	consumer := &Consumer{receiver: receiver}
	value, err := proto.Marshal(&eventspb.Event{
		EventId:     "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		AggregateId: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	})
	if err != nil {
		t.Fatal(err)
	}
	record := &kgo.Record{Topic: topic, Key: []byte("wrong-key"), Value: value}
	if _, _, err := consumer.apply(context.Background(), record); err == nil {
		t.Fatal("expected key mismatch")
	}
	if receiver.calls != 0 {
		t.Fatalf("receiver called %d times", receiver.calls)
	}
	record.Key = []byte("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	if _, _, err := consumer.apply(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if receiver.calls != 1 {
		t.Fatalf("receiver called %d times", receiver.calls)
	}
}
