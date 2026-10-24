package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

func TestCurrentUserResponse(t *testing.T) {
	for _, role := range []string{model.RoleClient, model.RoleAnalyst, model.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			id := uuid.New()
			user := model.CurrentUser{ID: id, Email: "user@example.test", FullName: "Иван Тестов", Role: role, Active: true}
			if role == model.RoleClient {
				user.CanSubmit = true
				user.Organization = &model.Organization{ID: uuid.New(), INN: "7707083893", Name: "Компания"}
			}
			handler := New(Params{
				Verifier: verifierFunc(func(_ context.Context, token string) (model.Principal, error) {
					if token != "valid" {
						t.Fatal("wrong token sent to verifier")
					}
					return model.Principal{UserID: id, Role: role}, nil
				}),
				Me: meFunc(func(_ context.Context, got uuid.UUID) (model.CurrentUser, error) {
					if got != id {
						t.Fatal("query parameters overrode the verified identity")
					}
					return user, nil
				}),
			})
			r := httptest.NewRequest(http.MethodGet, "/auth/me?user_id="+uuid.NewString()+"&org_id="+uuid.NewString(), nil)
			r.Header.Set("Authorization", "bearer valid")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			var body map[string]json.RawMessage
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil {
				t.Fatal("current user request failed")
			}
			var actualID, email, fullName, actualRole string
			var canSubmit bool
			if json.Unmarshal(body["id"], &actualID) != nil || json.Unmarshal(body["email"], &email) != nil || json.Unmarshal(body["full_name"], &fullName) != nil || json.Unmarshal(body["role"], &actualRole) != nil || json.Unmarshal(body["can_submit"], &canSubmit) != nil {
				t.Fatal("invalid response fields")
			}
			if actualID != id.String() || email != user.Email || fullName != user.FullName || actualRole != role || canSubmit != user.CanSubmit {
				t.Fatal("current user response mismatch")
			}
			if role == model.RoleClient {
				var org struct {
					ID   uuid.UUID `json:"id"`
					INN  string    `json:"inn"`
					Name string    `json:"name"`
				}
				if len(body) != 6 || json.Unmarshal(body["organization"], &org) != nil || org.ID != user.Organization.ID || org.INN != user.Organization.INN || org.Name != user.Organization.Name {
					t.Fatal("organization contract mismatch")
				}
			} else if len(body) != 5 || body["organization"] != nil {
				t.Fatal("staff response leaked an organization or extra fields")
			}
			if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("incorrect current user response headers")
			}
		})
	}
}

func TestCurrentUserRequiresSingleValidBearerToken(t *testing.T) {
	for _, headers := range [][]string{nil, {""}, {"Basic valid"}, {"Bearer"}, {"Bearer a b"}, {"Bearer rejected"}, {"Bearer valid", "Bearer other"}} {
		t.Run("invalid authorization", func(t *testing.T) {
			handler := New(Params{
				Verifier: verifierFunc(func(context.Context, string) (model.Principal, error) {
					return model.Principal{}, errors.New("private-token-error")
				}),
				Me: meFunc(func(context.Context, uuid.UUID) (model.CurrentUser, error) {
					t.Fatal("unauthenticated request reached the usecase")
					return model.CurrentUser{}, nil
				}),
			})
			r := httptest.NewRequest(http.MethodGet, "/auth/me?access_token=valid", nil)
			for _, header := range headers {
				r.Header.Add("Authorization", header)
			}
			r.AddCookie(&http.Cookie{Name: "refresh", Value: "valid"})
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			assertProblem(t, w, 401, "UNAUTHORIZED")
			if w.Header().Get("WWW-Authenticate") != "Bearer" || strings.Contains(w.Body.String(), "private-token-error") {
				t.Fatal("missing challenge or leaked verifier details")
			}
		})
	}
	for _, verifier := range []verifierFunc{nil, func(context.Context, string) (model.Principal, error) { return model.Principal{}, nil }} {
		params := Params{}
		if verifier != nil {
			params.Verifier = verifier
		}
		r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		r.Header.Set("Authorization", "Bearer valid")
		w := httptest.NewRecorder()
		New(params).ServeHTTP(w, r)
		assertProblem(t, w, 401, "UNAUTHORIZED")
	}
}

func TestCurrentUserUsecaseErrors(t *testing.T) {
	for _, err := range []error{domain.ErrUnauthorized, domain.ErrUnavailable, errors.New("private-database-error")} {
		handler := New(Params{
			Verifier: verifierFunc(func(context.Context, string) (model.Principal, error) {
				return model.Principal{UserID: uuid.New()}, nil
			}),
			Me: meFunc(func(context.Context, uuid.UUID) (model.CurrentUser, error) { return model.CurrentUser{}, err }),
		})
		r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		r.Header.Set("Authorization", "Bearer valid")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if errors.Is(err, domain.ErrUnauthorized) {
			assertProblem(t, w, 401, "UNAUTHORIZED")
			if w.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing challenge")
			}
		} else if errors.Is(err, domain.ErrUnavailable) {
			assertProblem(t, w, 503, "UNAVAILABLE")
		} else {
			assertProblem(t, w, 500, "INTERNAL_ERROR")
		}
		if strings.Contains(w.Body.String(), "private-database-error") {
			t.Fatal("database error was disclosed")
		}
	}
}

func TestRoutingErrorsUseProblemResponses(t *testing.T) {
	handler := New(Params{})
	for _, tc := range []struct {
		method, path string
		status       int
		code, allow  string
	}{
		{http.MethodGet, "/missing", 404, "NOT_FOUND", ""},
		{http.MethodPost, "/auth/me", 405, "METHOD_NOT_ALLOWED", "GET"},
		{http.MethodGet, "/auth/login", 405, "METHOD_NOT_ALLOWED", "POST"},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		assertProblem(t, w, tc.status, tc.code)
		if w.Header().Get("Allow") != tc.allow {
			t.Fatal("incorrect Allow header")
		}
	}
}
