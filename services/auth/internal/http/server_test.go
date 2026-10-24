package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/http/health"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestTechnicalRoutes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		db     health.Pinger
		status int
	}{
		{"liveness without DB", "/healthz", nil, 204},
		{"readiness without DB", "/readyz", nil, 503},
		{"database ready", "/readyz", pingerFunc(func(context.Context) error { return nil }), 204},
		{"database failure", "/readyz", pingerFunc(func(context.Context) error { return errors.New("private database details") }), 503},
		{"readiness timeout", "/readyz", pingerFunc(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }), 503},
		{"liveness skips DB", "/healthz", pingerFunc(func(context.Context) error { t.Fatal("liveness pinged DB"); return nil }), 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := New(Params{DB: tc.db, ReadinessTimeout: 10 * time.Millisecond})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != tc.status || response.Body.Len() != 0 || response.Header().Get("X-Trace-ID") == "" {
				t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
		})
	}
}

func TestBusinessRoutesAreExplicitlyUnimplemented(t *testing.T) {
	handler := New(Params{ReadinessTimeout: time.Second})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/auth/me", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var problem common.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusNotImplemented || problem.Code != "NOT_IMPLEMENTED" || problem.TraceId != response.Header().Get("X-Trace-ID") || response.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("status=%d problem=%+v", response.Code, problem)
			}
		})
	}
}

func TestMalformedRequestUsesProblemResponse(t *testing.T) {
	handler := New(Params{ReadinessTimeout: time.Second})
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var problem common.Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || problem.Code != "BAD_REQUEST" || problem.TraceId != response.Header().Get("X-Trace-ID") {
		t.Fatalf("status=%d problem=%+v", response.Code, problem)
	}
}
