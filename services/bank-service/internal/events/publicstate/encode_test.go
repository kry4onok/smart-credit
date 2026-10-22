package publicstate

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestUnderReviewEvent(t *testing.T) {
	eventID, caseID, key := uuid.New(), uuid.New(), uuid.New()
	when := time.Date(2026, time.October, 22, 12, 30, 45, 123000000, time.UTC)
	event := UnderReview(eventID, caseID, key, 3, when)
	fields := parseFields(t, event)
	if string(fields[1]) != eventID.String() || string(fields[2]) != "APPLICATION" ||
		string(fields[3]) != caseID.String() || string(fields[6]) != key.String() {
		t.Fatalf("event identifiers or aggregate type do not match")
	}
	if value, n := protowire.ConsumeVarint(fields[4]); n < 0 || value != 3 {
		t.Fatalf("aggregate_version = %d, want 3", value)
	}
	timestamp := parseFields(t, fields[5])
	if seconds, n := protowire.ConsumeVarint(timestamp[1]); n < 0 || seconds != uint64(when.Unix()) {
		t.Fatalf("occurred_at seconds = %d", seconds)
	}
	if nanos, n := protowire.ConsumeVarint(timestamp[2]); n < 0 || nanos != uint64(when.Nanosecond()) {
		t.Fatalf("occurred_at nanos = %d", nanos)
	}
	state := parseFields(t, fields[16])
	if status, n := protowire.ConsumeVarint(state[1]); n < 0 || status != 2 {
		t.Fatalf("client status = %d, want UNDER_REVIEW (2)", status)
	}
	if string(state[2]) != "Заявка на рассмотрении" || string(state[3]) != "Ожидайте решения банка" {
		t.Fatalf("public state text is incomplete")
	}
}

func parseFields(t *testing.T, wire []byte) map[protowire.Number][]byte {
	t.Helper()
	result := make(map[protowire.Number][]byte)
	for len(wire) > 0 {
		field, kind, n := protowire.ConsumeTag(wire)
		if n < 0 {
			t.Fatalf("invalid protobuf tag")
		}
		wire = wire[n:]
		var value []byte
		switch kind {
		case protowire.BytesType:
			var consumed int
			value, consumed = protowire.ConsumeBytes(wire)
			if consumed < 0 {
				t.Fatalf("invalid bytes field %d", field)
			}
			wire = wire[consumed:]
		case protowire.VarintType:
			_, consumed := protowire.ConsumeVarint(wire)
			if consumed < 0 {
				t.Fatalf("invalid varint field %d", field)
			}
			value = wire[:consumed]
			wire = wire[consumed:]
		default:
			t.Fatalf("unexpected wire type %d", kind)
		}
		result[field] = value
	}
	return result
}
