package me

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	FetchCurrent(context.Context, uuid.UUID) (model.CurrentUser, error)
}
