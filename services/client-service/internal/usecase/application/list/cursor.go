package list

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

const (
	cursorVersion   = 1
	maxCursorLength = 512
)

type cursorPayload struct {
	Version   int       `json:"v"`
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

func encodeCursor(item model.ApplicationListItem) (string, error) {
	payload, err := json.Marshal(cursorPayload{
		Version: cursorVersion, CreatedAt: item.CreatedAt.UTC(), ID: item.ID,
	})
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCursor(value *string) (*Cursor, error) {
	if value == nil {
		return nil, nil
	}
	if *value == "" || len(*value) > maxCursorLength {
		return nil, ErrInvalidCursor
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(*value)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed base64", ErrInvalidCursor)
	}
	var payload cursorPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("%w: malformed payload", ErrInvalidCursor)
	}
	if payload.Version != cursorVersion || payload.CreatedAt.IsZero() || payload.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: unsupported or incomplete payload", ErrInvalidCursor)
	}
	return &Cursor{CreatedAt: payload.CreatedAt, ID: payload.ID}, nil
}
