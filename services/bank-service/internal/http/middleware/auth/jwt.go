package authmiddleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/jwt"
	"github.com/google/uuid"
)

type Verifier interface {
	Verify(context.Context, string) (jwt.Principal, error)
}

func RequireEmployee(verifier Verifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		authorization := strings.Fields(r.Header.Get("Authorization"))
		if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") || verifier == nil {
			unauthorized(w, r)
			return
		}
		principal, err := verifier.Verify(r.Context(), authorization[1])
		if err != nil || principal.UserID == uuid.Nil {
			unauthorized(w, r)
			return
		}
		if principal.Role != "ANALYST" && principal.Role != "ADMIN" {
			domainerror.Write(w, r, domainerror.Forbidden("FORBIDDEN", errors.New("требуется роль ANALYST или ADMIN")))
			return
		}
		next.ServeHTTP(w, r.WithContext(WithAnalystID(r.Context(), principal.UserID)))
	})
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	domainerror.Write(w, r, &domainerror.Error{
		Status: http.StatusUnauthorized,
		Code:   "UNAUTHORIZED",
		Title:  "Требуется авторизация",
		Detail: "Требуется действительная сессия.",
	})
}
