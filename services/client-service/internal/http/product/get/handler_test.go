package get

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

func TestHandleReturnsProduct(t *testing.T) {
	id := uuid.New()
	usecase := &usecaseMock{product: model.Product{
		ID: id, Version: 2, Code: "EXPRESS", Name: "Экспресс", Description: "Оборотный кредит",
		PurposeTags: []string{"WORKING_CAPITAL"}, MinAmountKopecks: 100, MaxAmountKopecks: 200,
		MinTermMonths: 3, MaxTermMonths: 12, RequiredDocumentTypes: []string{"BANK_STATEMENT"},
	}}
	response, err := New(usecase).Handle(withClient(t), generated.GetClientProductRequestObject{ProductId: common.Id(id)})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetClientProductResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body common.Product
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || usecase.id != id || body.Id != common.Id(id) || body.Code != "EXPRESS" ||
		body.MinAmount.AmountKopecks != 100 || body.MinAmount.Currency != common.MoneyCurrencyRUB {
		t.Fatalf("status = %d, id = %s, body = %+v", recorder.Code, usecase.id, body)
	}
}

func TestHandleMapsProductNotFound(t *testing.T) {
	usecase := &usecaseMock{err: domain.ErrProductNotFound}
	response, err := New(usecase).Handle(withClient(t), generated.GetClientProductRequestObject{ProductId: common.Id(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetClientProductResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var problem common.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusNotFound || problem.Code != "PRODUCT_NOT_FOUND" {
		t.Fatalf("status = %d, problem = %+v", recorder.Code, problem)
	}
}

func TestHandleMapsUnexpectedError(t *testing.T) {
	usecase := &usecaseMock{err: errors.New("database unavailable")}
	response, err := New(usecase).Handle(withClient(t), generated.GetClientProductRequestObject{ProductId: common.Id(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetClientProductResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestHandleRequiresClient(t *testing.T) {
	usecase := &usecaseMock{}
	response, err := New(usecase).Handle(context.Background(), generated.GetClientProductRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetClientProductResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnauthorized || usecase.id != uuid.Nil {
		t.Fatalf("status = %d, id = %s", recorder.Code, usecase.id)
	}
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/client/products/00000000-0000-0000-0000-000000000001", nil)
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
