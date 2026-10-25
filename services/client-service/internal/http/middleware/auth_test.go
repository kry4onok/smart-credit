package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func TestRequireClient(t *testing.T) {
	for _, tc := range []struct {
		name      string
		header    string
		principal model.Principal
		err       error
		status    int
	}{
		{"missing token", "", model.Principal{}, nil, 401},
		{"wrong scheme", "Basic test", model.Principal{}, nil, 401},
		{"extra token", "Bearer one two", model.Principal{}, nil, 401},
		{"invalid signature", "Bearer test", model.Principal{}, errors.New("invalid"), 401},
		{"analyst", "Bearer test", model.Principal{UserID: uuid.New(), Role: "ANALYST"}, nil, 403},
		{"missing organization", "Bearer test", model.Principal{UserID: uuid.New(), Role: "CLIENT"}, nil, 401},
		{"client", "bearer test", model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}, nil, 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := RequireClient(verifierMock{tc.principal, tc.err}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if p, ok := Client(r.Context()); !ok || p != tc.principal {
					t.Fatal("missing principal")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			r := httptest.NewRequest(http.MethodGet, "/client/company", nil)
			r.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if tc.status == 401 && w.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing challenge")
			}
		})
	}
}

func TestRequireStaffContextAndMissingVerifier(t *testing.T) {
	for _, role := range []string{"ANALYST", "ADMIN"} {
		principal := model.Principal{UserID: uuid.New(), Role: role}
		handler := RequireStaff(verifierMock{principal: principal}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got, ok := Staff(r.Context()); !ok || got != principal {
				t.Fatal("verified staff principal missing from context")
			}
			if _, ok := Client(r.Context()); ok {
				t.Fatal("staff principal accepted as client")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		request := httptest.NewRequest(http.MethodGet, "/internal/client/applications", nil)
		request.Header.Set("Authorization", "Bearer test")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("staff status=%d", recorder.Code)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/internal/client/applications", nil)
	request.Header.Set("Authorization", "Bearer test")
	recorder := httptest.NewRecorder()
	RequireStaff(nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("missing verifier allowed access")
	})).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing verifier status=%d", recorder.Code)
	}
}
