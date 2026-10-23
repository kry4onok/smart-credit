package model

import (
	"time"

	"github.com/google/uuid"
)

type Application struct {
	ID             uuid.UUID `json:"id"`
	ProductID      uuid.UUID `json:"product_id"`
	ProductVersion int64     `json:"product_version"`
	Company        Company   `json:"company"`
	Status         string    `json:"status"`
	StageLabel     string    `json:"stage_label"`
	NextAction     string    `json:"next_action"`
	AllowedActions []string  `json:"allowed_actions"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Version        int64     `json:"version"`
}

type CreateApplicationParams struct {
	OrgID         uuid.UUID
	UserID        uuid.UUID
	ProductID     uuid.UUID
	IdempotencyID uuid.UUID
}
