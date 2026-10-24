package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type loginFunc func(context.Context, model.LoginParams) (model.LoginResult, error)

func (f loginFunc) Execute(ctx context.Context, p model.LoginParams) (model.LoginResult, error) {
	return f(ctx, p)
}

type refreshFunc func(context.Context, string) (model.AccessToken, error)

func (f refreshFunc) Execute(ctx context.Context, token string) (model.AccessToken, error) {
	return f(ctx, token)
}

type logoutFunc func(context.Context, string) error

func (f logoutFunc) Execute(ctx context.Context, token string) error { return f(ctx, token) }

type verifierFunc func(context.Context, string) (model.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token string) (model.Principal, error) {
	return f(ctx, token)
}

type meFunc func(context.Context, uuid.UUID) (model.CurrentUser, error)

func (f meFunc) Execute(ctx context.Context, id uuid.UUID) (model.CurrentUser, error) {
	return f(ctx, id)
}
