package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Verifier interface {
	Verify(context.Context, string) (model.Principal, error)
}

type principalKey struct{}

func Client(ctx context.Context) (model.Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(model.Principal)
	return p, ok && p.Role == "CLIENT" && p.OrgID != uuid.Nil && p.UserID != uuid.Nil
}

func RequireClient(verifier Verifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := strings.Fields(r.Header.Get("Authorization"))
		if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") || verifier == nil {
			unauthorized(w, r)
			return
		}
		p, err := verifier.Verify(r.Context(), authorization[1])
		if err != nil || p.UserID == uuid.Nil {
			unauthorized(w, r)
			return
		}
		if p.Role != "CLIENT" {
			domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusForbidden, "FORBIDDEN", "Требуется роль CLIENT."))
			return
		}
		if p.OrgID == uuid.Nil {
			unauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	domainerror.Write(w, domainerror.Problem(r.Context(), http.StatusUnauthorized, "UNAUTHORIZED", "Требуется действительная сессия."))
}
