package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

func TestRequestLogsExcludeSensitiveData(t *testing.T) {
	var logs bytes.Buffer
	traceID := uuid.NewString()
	userID := uuid.New()
	handler := New(Params{Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		Verifier: verifierFunc(func(context.Context, string) (model.Principal, error) { return model.Principal{UserID: userID}, nil }),
		Me: meFunc(func(context.Context, uuid.UUID) (model.CurrentUser, error) {
			return model.CurrentUser{ID: userID, Email: "private-email@example.test", FullName: "private-full-name", Role: model.RoleClient,
				Organization: &model.Organization{ID: uuid.New(), INN: "7707083893", Name: "private-company"}}, nil
		}),
	})
	r := httptest.NewRequest(http.MethodGet, "/auth/me?access_token=private-query-token", strings.NewReader("private-body-password"))
	r.Header.Set("Authorization", "Bearer private-access-token")
	r.Header.Set("Cookie", "refresh=private-refresh-token")
	r.Header.Set("X-Trace-ID", traceID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("X-Trace-ID") != traceID {
		t.Fatal("request failed or lost trace ID")
	}
	var event map[string]any
	if json.Unmarshal(logs.Bytes(), &event) != nil || event["msg"] != "auth request" || event["route"] != "/auth/me" || event["status"] != float64(200) || event["trace_id"] != traceID || event["method"] != "GET" {
		t.Fatal("request log is missing safe diagnostic fields")
	}
	for _, secret := range []string{"private-", "7707083893", userID.String()} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("request log disclosed sensitive data")
		}
	}
	logs.Reset()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/private-unmatched-token?secret=private-query", nil))
	if strings.Contains(logs.String(), "private-") || !strings.Contains(logs.String(), "<unmatched>") {
		t.Fatal("unmatched URL was logged verbatim")
	}
}

func TestPanicBecomesSanitizedProblemResponse(t *testing.T) {
	var logs bytes.Buffer
	handler := New(Params{Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		Verifier: verifierFunc(func(context.Context, string) (model.Principal, error) {
			return model.Principal{UserID: uuid.New()}, nil
		}),
		Me: meFunc(func(context.Context, uuid.UUID) (model.CurrentUser, error) { panic("private-panic-password-and-token") }),
	})
	r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	r.Header.Set("Authorization", "Bearer valid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	assertProblem(t, w, 500, "INTERNAL_ERROR")
	if strings.Contains(w.Body.String(), "private-panic") || strings.Contains(logs.String(), "private-panic") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("panic disclosed details or became cacheable")
	}
	if !strings.Contains(logs.String(), "auth request panic") || !strings.Contains(logs.String(), `"status":500`) {
		t.Fatal("panic was not recorded with the failure status")
	}
}

func TestTraceHeadersAreValidated(t *testing.T) {
	id := uuid.NewString()
	handler := New(Params{})
	for _, values := range [][]string{{id}, {"private-invalid-id"}, {uuid.Nil.String()}, {id, uuid.NewString()}, nil} {
		r := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		for _, value := range values {
			r.Header.Add("X-Trace-ID", value)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		assertProblem(t, w, 401, "UNAUTHORIZED")
		got, err := uuid.Parse(w.Header().Get("X-Trace-ID"))
		if err != nil || got == uuid.Nil {
			t.Fatal("invalid trace ID emitted")
		}
		if len(values) == 1 && values[0] == id {
			if got.String() != id {
				t.Fatal("valid incoming trace ID not propagated")
			}
		} else if got.String() == id || strings.Contains(w.Body.String(), "private-invalid-id") {
			t.Fatal("invalid or ambiguous incoming trace ID was trusted")
		}
	}
}
