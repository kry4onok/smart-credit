package list

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

func TestHandleListsProductsAndPassesPurpose(t *testing.T) {
	purpose := common.WORKINGCAPITAL
	usecase := &usecaseMock{catalog: model.ProductCatalog{
		Version: 3,
		Items: []model.Product{{
			ID: uuid.New(), Version: 2, Code: "EXPRESS", Name: "Экспресс",
			Description: "Оборотный кредит", PurposeTags: []string{"WORKING_CAPITAL"},
			MinAmountKopecks: 100, MaxAmountKopecks: 200, MinTermMonths: 3, MaxTermMonths: 12,
			RequiredDocumentTypes: []string{"BANK_STATEMENT"},
		}},
	}}
	handler := New(usecase)
	response, err := handler.Handle(withClient(t), generated.ListClientProductsRequestObject{
		Params: generated.ListClientProductsParams{Purpose: &purpose},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitListClientProductsResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || usecase.purpose == nil || *usecase.purpose != string(purpose) {
		t.Fatalf("status = %d, purpose = %v", recorder.Code, usecase.purpose)
	}
	var body generated.ListClientProducts200JSONResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.CatalogVersion != 3 || len(body.Items) != 1 || body.Items[0].Code != "EXPRESS" ||
		body.Items[0].MinAmount.AmountKopecks != 100 || body.Items[0].MinAmount.Currency != common.MoneyCurrencyRUB {
		t.Fatalf("body = %+v", body)
	}
}

func TestHandleRejectsUnknownPurpose(t *testing.T) {
	value := common.PurposeTag("UNKNOWN")
	usecase := &usecaseMock{}
	response, err := New(usecase).Handle(withClient(t), generated.ListClientProductsRequestObject{
		Params: generated.ListClientProductsParams{Purpose: &value},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitListClientProductsResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var problem common.Problem
	if json.Unmarshal(recorder.Body.Bytes(), &problem) != nil || recorder.Code != http.StatusBadRequest || problem.Code != "INVALID_PURPOSE" || usecase.purpose != nil {
		t.Fatalf("status = %d, problem = %+v, purpose = %v", recorder.Code, problem, usecase.purpose)
	}
}

func TestHandleRequiresClient(t *testing.T) {
	usecase := &usecaseMock{}
	response, err := New(usecase).Handle(context.Background(), generated.ListClientProductsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitListClientProductsResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnauthorized || usecase.purpose != nil {
		t.Fatalf("status = %d, purpose = %v", recorder.Code, usecase.purpose)
	}
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/client/products", nil)
	request.Header.Set("Authorization", "Bearer test")
	var captured context.Context
	middleware.RequireClient(verifierMock{principal: model.Principal{
		UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT",
	}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), request)
	if captured == nil {
		t.Fatal("principal context was not captured")
	}
	return captured
}
