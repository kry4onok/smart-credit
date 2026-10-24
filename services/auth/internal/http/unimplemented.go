package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
)

func (s *server) Login(context.Context, generated.LoginRequestObject) (generated.LoginResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) RefreshSession(context.Context, generated.RefreshSessionRequestObject) (generated.RefreshSessionResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) Logout(context.Context, generated.LogoutRequestObject) (generated.LogoutResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetCurrentUser(context.Context, generated.GetCurrentUserRequestObject) (generated.GetCurrentUserResponseObject, error) {
	return nil, errNotImplemented
}

func (s *server) GetJwks(context.Context, generated.GetJwksRequestObject) (generated.GetJwksResponseObject, error) {
	return nil, errNotImplemented
}
