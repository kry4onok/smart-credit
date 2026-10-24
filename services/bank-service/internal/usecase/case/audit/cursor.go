package audit

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const cursorVersion = 1
const maxCursorLength = 512

type cursorPayload struct {
	Version    int       `json:"v"`
	CaseID     uuid.UUID `json:"case_id"`
	OccurredAt time.Time `json:"occurred_at"`
	ID         uuid.UUID `json:"id"`
}

func encodeCursor(caseID uuid.UUID, entry Entry) (string, error) {
	payload, err := json.Marshal(cursorPayload{Version: cursorVersion, CaseID: caseID, OccurredAt: entry.OccurredAt.UTC(), ID: entry.ID})
	if err != nil {
		return "", fmt.Errorf("encode audit cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
func decodeCursor(caseID uuid.UUID, value *string) (*Cursor, error) {
	if value == nil {
		return nil, nil
	}
	if *value == "" || len(*value) > maxCursorLength {
		return nil, ErrInvalidCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(*value)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed base64", ErrInvalidCursor)
	}
	var payload cursorPayload
	if err = json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: malformed payload", ErrInvalidCursor)
	}
	if payload.Version != cursorVersion || payload.CaseID != caseID || payload.OccurredAt.IsZero() || payload.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: cursor does not belong to this case", ErrInvalidCursor)
	}
	return &Cursor{OccurredAt: payload.OccurredAt, ID: payload.ID}, nil
}
