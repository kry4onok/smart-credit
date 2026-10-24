package me

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type repositoryMock struct {
	user  model.CurrentUser
	err   error
	id    uuid.UUID
	calls int
}

func (m *repositoryMock) FetchCurrent(_ context.Context, id uuid.UUID) (model.CurrentUser, error) {
	m.id = id
	m.calls++
	return m.user, m.err
}
