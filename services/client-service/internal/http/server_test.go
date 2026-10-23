package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	applicationcreate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/create"
	applicationget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/get"
	companyget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/company/get"
	productget "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/get"
	productlist "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/product/list"
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

type productsMock struct{}

func (productsMock) Execute(context.Context, *string) (model.ProductCatalog, error) {
	return model.ProductCatalog{Version: 1, Items: []model.Product{}}, nil
}

type productMock struct{}

func (productMock) Execute(_ context.Context, id uuid.UUID) (model.Product, error) {
	return model.Product{ID: id, Version: 1, Code: "EXPRESS", Name: "Экспресс"}, nil
}

type applicationMock struct{}

func (applicationMock) Execute(_ context.Context, orgID, userID, productID, idempotencyID uuid.UUID) (model.Application, error) {
	return model.Application{ID: uuid.New(), ProductID: productID, ProductVersion: 1, Company: model.Company{
		INN: "7707083893", Name: "Demo", OKVED: "62.01", Version: 1,
		RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}, Status: "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 1}, nil
}

type applicationGetMock struct{}

func (applicationGetMock) Execute(_ context.Context, _ uuid.UUID, applicationID uuid.UUID) (model.Application, error) {
	return model.Application{ID: applicationID, ProductID: uuid.New(), ProductVersion: 1, Company: model.Company{
		INN: "7707083893", Name: "Demo", OKVED: "62.01",
		RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}, Status: "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 1}, nil
}

func TestRoutes(t *testing.T) {
	handler := New(pingerMock{}, time.Second, companyget.New(companyMock{}), nil, applicationcreate.New(applicationMock{}), applicationget.New(applicationGetMock{}), productget.New(productMock{}), productlist.New(productsMock{}), verifierMock{model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}})
	for _, tc := range []struct {
		path, authorization string
		status              int
	}{
		{"/healthz", "", 204}, {"/readyz", "", 204}, {"/client/company", "", 401},
		{"/client/company", "Bearer test", 200},
		{"/client/products", "Bearer test", 200},
		{"/client/products/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
		{"/client/applications", "Bearer test", 201},
		{"/client/applications/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
	} {
		t.Run(tc.path+tc.authorization, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.path == "/client/applications" {
				request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"product_id":"00000000-0000-0000-0000-000000000001"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000002")
			}
			request.Header.Set("Authorization", tc.authorization)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
