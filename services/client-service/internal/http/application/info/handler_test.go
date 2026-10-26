package info

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

func TestHandleAcceptsInfoResponse(t *testing.T) {
	applicationID, requestID := uuid.New(), uuid.New()
	handler := New(usecaseMock{accepted: model.ActionAccepted{ApplicationID: applicationID, PendingAction: "INFO_SENT"}})
	term := 18
	response, err := handler.Handle(withClient(t), generated.ProvideInfoRequestObject{
		ApplicationId: common.Id(applicationID),
		Params:        generated.ProvideInfoParams{IdempotencyKey: common.IdempotencyKey(uuid.New())},
		Body: &generated.ProvideInfoJSONRequestBody{
			RequestId: common.Id(requestID),
			Fields:    &generated.LoanFieldsPatch{TermMonths: &term},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitProvideInfoResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body generated.ActionAccepted
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusAccepted || body.ApplicationId != common.Id(applicationID) || body.PendingAction != "INFO_SENT" {
		t.Fatalf("status = %d, body = %+v", recorder.Code, body)
	}
}

func TestHandleRejectsMissingBody(t *testing.T) {
	response, err := New(usecaseMock{}).Handle(withClient(t), generated.ProvideInfoRequestObject{
		ApplicationId: common.Id(uuid.New()),
		Params:        generated.ProvideInfoParams{IdempotencyKey: common.Id(uuid.New())},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitProvideInfoResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", recorder.Code)
	}
}

type usecaseMock struct{ accepted model.ActionAccepted }

func (m usecaseMock) Execute(context.Context, model.ProvideInfoParams) (model.ActionAccepted, error) {
	return m.accepted, nil
}

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/client/applications/1/info", nil)
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
