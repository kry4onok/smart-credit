package create

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestHandleCreatesDraft(t *testing.T) {
	applicationID, productID, idempotencyID := uuid.New(), uuid.New(), uuid.New()
	usecase := &usecaseMock{application: model.Application{
		ID: applicationID, ProductID: productID, ProductVersion: 2,
		Company: model.Company{INN: "7707083893", Name: "Demo", OKVED: "62.01", RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		Status:  "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 1,
	}}
	response, err := New(usecase).Handle(withClient(t), generated.CreateApplicationRequestObject{
		Params: generated.CreateApplicationParams{IdempotencyKey: common.IdempotencyKey(idempotencyID)},
		Body:   &generated.CreateApplicationJSONRequestBody{ProductId: common.Id(productID)},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitCreateApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body generated.ClientApplication
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusCreated || !usecase.called || body.Id != common.Id(applicationID) || body.ProductId != common.Id(productID) ||
		body.Status != common.ClientStatus("DRAFT") || body.Company.Inn != "7707083893" {
		t.Fatalf("status = %d, called = %v, body = %+v", recorder.Code, usecase.called, body)
	}
}

func TestHandleRejectsInvalidRequest(t *testing.T) {
	usecase := &usecaseMock{}
	response, err := New(usecase).Handle(withClient(t), generated.CreateApplicationRequestObject{
		Params: generated.CreateApplicationParams{IdempotencyKey: common.IdempotencyKey(uuid.New())},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitCreateApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnprocessableEntity || usecase.called {
		t.Fatalf("status = %d, called = %v", recorder.Code, usecase.called)
	}
}

func TestHandleMapsErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "product missing", err: domain.ErrProductNotFound, status: http.StatusNotFound, code: "PRODUCT_NOT_FOUND"},
		{name: "idempotency conflict", err: domain.ErrIdempotencyConflict, status: http.StatusConflict, code: "IDEMPOTENCY_CONFLICT"},
		{name: "database failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError, code: "INTERNAL_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &usecaseMock{err: tc.err}
			response, err := New(usecase).Handle(withClient(t), validRequest())
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if err := response.VisitCreateApplicationResponse(recorder); err != nil {
				t.Fatal(err)
			}
			var problem common.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || problem.Code != tc.code {
				t.Fatalf("status = %d, problem = %+v", recorder.Code, problem)
			}
		})
	}
}

func validRequest() generated.CreateApplicationRequestObject {
	return generated.CreateApplicationRequestObject{
		Params: generated.CreateApplicationParams{IdempotencyKey: common.IdempotencyKey(uuid.New())},
		Body:   &generated.CreateApplicationJSONRequestBody{ProductId: common.Id(uuid.New())},
	}
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/client/applications", nil)
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
