package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	companyget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/get"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type pingerMock struct{}

func (pingerMock) Ping(context.Context) error { return nil }

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

type companyMock struct{}

func (companyMock) Execute(context.Context, uuid.UUID) (model.Company, error) {
	return model.Company{INN: "7707083893", Name: "Demo", OKVED: "62.01", Version: 1, RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func TestRoutes(t *testing.T) {
	handler := New(pingerMock{}, time.Second, companyget.New(companyMock{}), nil, verifierMock{model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}})
	for _, tc := range []struct {
		path, authorization string
		status              int
	}{
		{"/healthz", "", 204}, {"/readyz", "", 204}, {"/client/company", "", 401},
		{"/client/company", "Bearer test", 200},
	} {
		t.Run(tc.path+tc.authorization, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("Authorization", tc.authorization)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
