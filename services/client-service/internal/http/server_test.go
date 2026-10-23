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
	applicationupdate "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/update"
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

type applicationUpdateMock struct{}

func (applicationUpdateMock) Execute(_ context.Context, _ uuid.UUID, applicationID uuid.UUID, _ model.ApplicationPatch, _ int64) (model.Application, error) {
	return model.Application{ID: applicationID, ProductID: uuid.New(), ProductVersion: 1, Company: model.Company{
		INN: "7707083893", Name: "Demo", OKVED: "62.01",
		RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}, Status: "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 2}, nil
}

func TestRoutes(t *testing.T) {
	handler := New(pingerMock{}, time.Second, companyget.New(companyMock{}), nil, applicationcreate.New(applicationMock{}), applicationget.New(applicationGetMock{}), applicationupdate.New(applicationUpdateMock{}), productget.New(productMock{}), productlist.New(productsMock{}), verifierMock{model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}})
	for _, tc := range []struct {
		method, path, authorization string
		status                      int
	}{
		{http.MethodGet, "/healthz", "", 204}, {http.MethodGet, "/readyz", "", 204}, {http.MethodGet, "/client/company", "", 401},
		{http.MethodGet, "/client/company", "Bearer test", 200},
		{http.MethodGet, "/client/products", "Bearer test", 200},
		{http.MethodGet, "/client/products/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
		{http.MethodPost, "/client/applications", "Bearer test", 201},
		{http.MethodGet, "/client/applications/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
		{http.MethodPatch, "/client/applications/00000000-0000-0000-0000-000000000001", "Bearer test", 200},
	} {
		t.Run(tc.method+tc.path+tc.authorization, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.path == "/client/applications" {
				request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"product_id":"00000000-0000-0000-0000-000000000001"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000002")
			} else if tc.method == http.MethodPatch {
				request = httptest.NewRequest(http.MethodPatch, tc.path, strings.NewReader(`{"amount":{"amount_kopecks":100,"currency":"RUB"}}`))
				request.Header.Set("Content-Type", "application/merge-patch+json")
				request.Header.Set("If-Match", `"1"`)
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
