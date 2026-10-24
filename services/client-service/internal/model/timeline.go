package model

import (
	"time"

	"github.com/google/uuid"
)

type TimelineEntry struct {
	ID         uuid.UUID
	Status     string
	Message    string
	OccurredAt time.Time
}
