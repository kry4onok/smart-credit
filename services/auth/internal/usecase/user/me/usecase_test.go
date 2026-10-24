package me

import (
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

func TestCurrentUserForAllRoles(t *testing.T) {
	for _, role := range []string{model.RoleClient, model.RoleAnalyst, model.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			user := model.CurrentUser{ID: uuid.New(), Email: "user@example.test", FullName: "Тест", Role: role, Active: true}
			if role == model.RoleClient {
				user.Organization = &model.Organization{ID: uuid.New(), INN: "7707083893", Name: "Компания"}
				user.CanSubmit = true
			}
			repository := &repositoryMock{user: user}
			got, err := New(repository).Execute(t.Context(), user.ID)
			if err != nil || got != user || repository.id != user.ID {
				t.Fatal("current user was not fetched by verified subject")
			}
		})
	}
}

func TestCurrentUserRejections(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name      string
		id        uuid.UUID
		user      model.CurrentUser
		err, want error
	}{
		{"missing identity", uuid.Nil, model.CurrentUser{}, nil, domain.ErrUnauthorized},
		{"deleted", uuid.New(), model.CurrentUser{}, domain.ErrUserNotFound, domain.ErrUnauthorized},
		{"inactive", uuid.New(), model.CurrentUser{Active: false}, nil, domain.ErrUnauthorized},
		{"database error", uuid.New(), model.CurrentUser{}, failure, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &repositoryMock{user: tc.user, err: tc.err}
			got, err := New(repository).Execute(t.Context(), tc.id)
			if !errors.Is(err, tc.want) || got != (model.CurrentUser{}) {
				t.Fatal("invalid identity returned a profile or swallowed an error")
			}
			if tc.id == uuid.Nil && repository.calls != 0 {
				t.Fatal("missing identity reached the database")
			}
		})
	}
}
