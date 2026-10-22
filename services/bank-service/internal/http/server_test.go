package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type pingerStub struct {
	err   error
	calls int
}

func (p *pingerStub) Ping(context.Context) error {
	p.calls++
	return p.err
}

func TestHealth(t *testing.T) {
	db := &pingerStub{err: errors.New("database is unavailable")}
	handler := New(db, time.Second, nil, nil, nil)

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
			handler := New(db, time.Second, nil, nil, nil)

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

func TestUnimplementedContractRoute(t *testing.T) {
	handler := New(&pingerStub{}, time.Second, nil, nil, nil)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/backoffice/applications/00000000-0000-0000-0000-000000000001/audit", nil))

	if response.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotImplemented)
	}
}
