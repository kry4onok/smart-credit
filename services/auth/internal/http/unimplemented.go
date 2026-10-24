package httpapi

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
)

func (s *server) GetCurrentUser(context.Context, generated.GetCurrentUserRequestObject) (generated.GetCurrentUserResponseObject, error) {
	return nil, errNotImplemented
}
