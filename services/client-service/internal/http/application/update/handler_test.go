package update

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

func TestHandleUpdatesDraft(t *testing.T) {
	applicationID := uuid.New()
	usecase := &usecaseMock{application: model.Application{
		ID: applicationID, ProductID: uuid.New(), ProductVersion: 1,
		Company: model.Company{INN: "7707083893", Name: "Demo", OKVED: "62.01", RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		Status:  "DRAFT", StageLabel: "Черновик", NextAction: "Заполните заявку", Version: 2,
	}}
	amount := common.Money{AmountKopecks: 100, Currency: common.MoneyCurrencyRUB}
	term := 12
	purpose := common.PurposeTag("EQUIPMENT")
	response, err := New(usecase).Handle(withClient(t), generated.UpdateApplicationRequestObject{
		ApplicationId: common.Id(applicationID),
		Params:        generated.UpdateApplicationParams{IfMatch: `"1"`},
		Body:          &generated.ApplicationFormPatch{Amount: &amount, TermMonths: &term, Purpose: &purpose},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitUpdateApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || !usecase.called || usecase.version != 1 {
		t.Fatalf("status = %d, called = %v, version = %d", recorder.Code, usecase.called, usecase.version)
	}
}

func TestHandleRejectsInvalidIfMatch(t *testing.T) {
	usecase := &usecaseMock{}
	response, err := New(usecase).Handle(withClient(t), generated.UpdateApplicationRequestObject{Params: generated.UpdateApplicationParams{IfMatch: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitUpdateApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusBadRequest || usecase.called {
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
		{name: "not found", err: domain.ErrApplicationNotFound, status: http.StatusNotFound, code: "APPLICATION_NOT_FOUND"},
		{name: "version conflict", err: domain.ErrApplicationVersionConflict, status: http.StatusConflict, code: "APPLICATION_VERSION_CONFLICT"},
		{name: "state conflict", err: domain.ErrApplicationNotDraft, status: http.StatusConflict, code: "APPLICATION_STATE_CONFLICT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := New(&usecaseMock{err: tc.err}).Handle(withClient(t), validRequest())
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if err := response.VisitUpdateApplicationResponse(recorder); err != nil {
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

func validRequest() generated.UpdateApplicationRequestObject {
	term := 12
	return generated.UpdateApplicationRequestObject{
		ApplicationId: common.Id(uuid.New()),
		Params:        generated.UpdateApplicationParams{IfMatch: `"1"`},
		Body:          &generated.ApplicationFormPatch{TermMonths: &term},
	}
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodPatch, "/client/applications/1", nil)
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

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}
