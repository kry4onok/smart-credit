package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	summarycase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/summary"
	"github.com/google/uuid"
)

type pingerStub struct {
	err   error
	calls int
}

type summaryUseCaseStub struct {
	output summarycase.Output
	err    error
}

func (s summaryUseCaseStub) Execute(context.Context, uuid.UUID) (summarycase.Output, error) {
	return s.output, s.err
}

func (p *pingerStub) Ping(context.Context) error {
	p.calls++
	return p.err
}

func TestHealth(t *testing.T) {
	db := &pingerStub{err: errors.New("database is unavailable")}
	handler := New(db, time.Second, nil, nil, nil, nil, nil, nil)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if db.calls != 0 {
		t.Fatalf("database ping calls = %d, want 0", db.calls)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name       string
		pingError  error
		wantStatus int
	}{
		{name: "database is ready", wantStatus: http.StatusNoContent},
		{name: "database is unavailable", pingError: errors.New("connection refused"), wantStatus: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &pingerStub{err: tt.pingError}
			handler := New(db, time.Second, nil, nil, nil, nil, nil, nil)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil))

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if db.calls != 1 {
				t.Fatalf("database ping calls = %d, want 1", db.calls)
			}
		})
	}
}

func TestSummaryRouteRequiresEmployee(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil, nil)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/backoffice/applications/00000000-0000-0000-0000-000000000001/summary", nil))

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestSummaryRouteReturnsTextAndSource(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil,
		summaryUseCaseStub{output: summarycase.Output{Text: "Продукт: кредит.", Source: "template"}})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/backoffice/applications/10000000-0000-4000-8000-000000000001/summary", nil)
	request = request.WithContext(authmiddleware.WithAnalystID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Text   string `json:"text"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Text != "Продукт: кредит." || body.Source != "template" {
		t.Fatalf("body = %+v, err = %v", body, err)
	}
}

func TestSummaryRouteReturnsNotFound(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil,
		summaryUseCaseStub{err: summarycase.ErrNotFound})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/backoffice/applications/10000000-0000-4000-8000-000000000001/summary", nil)
	request = request.WithContext(authmiddleware.WithAnalystID(request.Context(), uuid.New()))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAuditRouteRequiresEmployee(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/backoffice/applications/10000000-0000-4000-8000-000000000001/audit", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

func TestTakeInvalidHeadersReturnProblemJSON(t *testing.T) {
	tests := []struct {
		name           string
		idempotencyKey string
		ifMatch        string
	}{
		{
			name:           "missing If-Match",
			idempotencyKey: "11111111-1111-4111-8111-111111111111",
		},
		{
			name:           "invalid Idempotency-Key",
			idempotencyKey: "not-a-uuid",
			ifMatch:        `"1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New(&pingerStub{}, time.Second, nil, nil, nil, nil, nil, nil)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
				"/backoffice/applications/10000000-0000-4000-8000-000000000001/take", nil)
			request.Header.Set("Idempotency-Key", tt.idempotencyKey)
			if tt.ifMatch != "" {
				request.Header.Set("If-Match", tt.ifMatch)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/problem+json" {
				t.Fatalf("Content-Type = %q, want application/problem+json", contentType)
			}
			var problem common.Problem
			if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Status != http.StatusBadRequest || problem.Code != "INVALID_REQUEST_PARAMETER" ||
				problem.Detail == nil || problem.TraceId == "" {
				t.Fatalf("problem = %+v", problem)
			}
		})
	}
}
