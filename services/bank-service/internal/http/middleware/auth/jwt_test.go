package authmiddleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/jwt"
	"github.com/google/uuid"
)

type verifierFunc func(context.Context, string) (jwt.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token string) (jwt.Principal, error) {
	return f(ctx, token)
}

func TestRequireEmployee(t *testing.T) {
	userID := uuid.New()
	var calls int
	verifier := verifierFunc(func(_ context.Context, token string) (jwt.Principal, error) {
		calls++
		switch token {
		case "analyst":
			return jwt.Principal{UserID: userID, Role: "ANALYST"}, nil
		case "admin":
			return jwt.Principal{UserID: userID, Role: "ADMIN"}, nil
		case "client":
			return jwt.Principal{UserID: userID, Role: "CLIENT"}, nil
		default:
			return jwt.Principal{}, errors.New("invalid token")
		}
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		id, ok := AnalystID(r.Context())
		if !ok || id != userID {
			t.Error("employee identity does not match JWT subject")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequireEmployee(verifier, next)
	for _, test := range []struct {
		name, path, authorization string
		status                    int
	}{
		{"health", "/healthz", "", http.StatusNoContent},
		{"readiness", "/readyz", "", http.StatusNoContent},
		{"missing", "/backoffice/applications", "", http.StatusUnauthorized},
		{"invalid", "/backoffice/applications", "Bearer invalid", http.StatusUnauthorized},
		{"wrong scheme", "/backoffice/applications", "Basic analyst", http.StatusUnauthorized},
		{"client role", "/backoffice/applications", "Bearer client", http.StatusForbidden},
		{"analyst role", "/backoffice/applications", "Bearer analyst", http.StatusNoContent},
		{"admin role", "/backoffice/applications", "Bearer admin", http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if test.status == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing Bearer challenge")
			}
		})
	}
	if calls != 4 {
		t.Fatalf("verifier calls = %d, want 4", calls)
	}
}
