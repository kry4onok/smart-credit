package submit

import (
	"context"
	"encoding/json"
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

func TestHandleSubmitsApplication(t *testing.T) {
	applicationID := uuid.New()
	usecase := &usecaseMock{application: model.Application{
		ID: applicationID, ProductID: uuid.New(), ProductVersion: 1,
		Company: model.Company{INN: "7707083893", Name: "Demo", OKVED: "62.01", RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		Form:    &model.ApplicationForm{AmountKopecks: 100_000, TermMonths: 12, Purpose: "EQUIPMENT"},
		Status:  "SUBMITTED", StageLabel: "Отправлена", NextAction: "Ожидайте решения банка", Version: 3,
	}}
	response, err := New(usecase).Handle(withClient(t), generated.SubmitApplicationRequestObject{
		ApplicationId: common.Id(applicationID),
		Params:        generated.SubmitApplicationParams{IdempotencyKey: common.IdempotencyKey(uuid.New())},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitSubmitApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || !usecase.called {
		t.Fatalf("status = %d, called = %v", recorder.Code, usecase.called)
	}
	var body generated.ClientApplication
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Id != common.Id(applicationID) || body.Status != common.ClientStatus("SUBMITTED") {
		t.Fatalf("body = %+v", body)
	}
}

func TestHandleRejectsMissingIdempotencyKey(t *testing.T) {
	usecase := &usecaseMock{}
	response, err := New(usecase).Handle(withClient(t), generated.SubmitApplicationRequestObject{ApplicationId: common.Id(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitSubmitApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusBadRequest || usecase.called {
		t.Fatalf("status = %d, called = %v", recorder.Code, usecase.called)
	}
}

func TestHandleMapsSubmissionErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code string
	}{
		{name: "not found", err: domain.ErrApplicationNotFound, code: "APPLICATION_NOT_FOUND"},
		{name: "not draft", err: domain.ErrApplicationNotDraft, code: "APPLICATION_STATE_CONFLICT"},
		{name: "invalid", err: domain.ErrInvalidApplication, code: "INVALID_APPLICATION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := New(&usecaseMock{err: tc.err}).Handle(withClient(t), validRequest())
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if err := response.VisitSubmitApplicationResponse(recorder); err != nil {
				t.Fatal(err)
			}
			var problem common.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if problem.Code != tc.code {
				t.Fatalf("status = %d, problem = %+v", recorder.Code, problem)
			}
		})
	}
}

func validRequest() generated.SubmitApplicationRequestObject {
	return generated.SubmitApplicationRequestObject{
		ApplicationId: common.Id(uuid.New()),
		Params:        generated.SubmitApplicationParams{IdempotencyKey: common.IdempotencyKey(uuid.New())},
	}
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/client/applications/1/submit", nil)
	request.Header.Set("Authorization", "Bearer test")
	var captured context.Context
	middleware.RequireClient(verifierMock{principal: model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), request)
	if captured == nil {
		t.Fatal("principal context was not captured")
	}
	return captured
}

type usecaseMock struct {
	application model.Application
	err         error
	called      bool
}

func (m *usecaseMock) Execute(context.Context, model.SubmitApplicationParams) (model.Application, error) {
	m.called = true
	return m.application, m.err
}

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}
