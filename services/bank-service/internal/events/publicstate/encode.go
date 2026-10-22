package publicstate

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protowire"
)

// UnderReview serializes proto/events/events.proto Event with a complete
// ApplicationPublicStatePublished snapshot. The fields below use the proto
// field numbers directly until generated protobuf types are added to the repo.
func UnderReview(eventID, caseID, correlationID uuid.UUID, version int64, occurredAt time.Time) []byte {
	var timestamp []byte
	timestamp = appendVarint(timestamp, 1, uint64(occurredAt.Unix()))
	if nanos := occurredAt.Nanosecond(); nanos != 0 {
		timestamp = appendVarint(timestamp, 2, uint64(nanos))
	}

	var state []byte
	state = appendVarint(state, 1, 2) // CLIENT_STATUS_UNDER_REVIEW
	state = appendString(state, 2, "Заявка на рассмотрении")
	state = appendString(state, 3, "Ожидайте решения банка")

	var event []byte
	event = appendString(event, 1, eventID.String())
	event = appendString(event, 2, "APPLICATION")
	event = appendString(event, 3, caseID.String())
	event = appendVarint(event, 4, uint64(version))
	event = appendMessage(event, 5, timestamp)
	event = appendString(event, 6, correlationID.String())
	event = appendMessage(event, 16, state)
	return event
}

func appendString(dst []byte, field protowire.Number, value string) []byte {
	return protowire.AppendString(protowire.AppendTag(dst, field, protowire.BytesType), value)
}

func appendVarint(dst []byte, field protowire.Number, value uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(dst, field, protowire.VarintType), value)
}

func appendMessage(dst []byte, field protowire.Number, value []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(dst, field, protowire.BytesType), value)
}
