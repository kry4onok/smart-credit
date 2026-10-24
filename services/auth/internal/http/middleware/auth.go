package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type Verifier interface {
	Verify(context.Context, string) (model.Principal, error)
}
type principalKey struct{}

func Principal(ctx context.Context) (model.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(model.Principal)
	return p, ok && p.UserID != uuid.Nil
}

func RequireBearer(verifier Verifier) generated.StrictMiddlewareFunc {
	return func(next generated.StrictHandlerFunc, operation string) generated.StrictHandlerFunc {
		if operation != "GetCurrentUser" {
			return next
		}
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			values := r.Header.Values("Authorization")
			var parts []string
			if len(values) == 1 {
				parts = strings.Fields(values[0])
			}
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || verifier == nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				return nil, domain.ErrUnauthorized
			}
			principal, err := verifier.Verify(ctx, parts[1])
			if err != nil || principal.UserID == uuid.Nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				return nil, domain.ErrUnauthorized
			}
			return next(context.WithValue(ctx, principalKey{}, principal), w, r, request)
		}
	}
}
