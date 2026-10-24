package sign

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

func TestHandleSignsContract(t *testing.T) {
	applicationID, offerID := uuid.New(), uuid.New()
	response, err := New(usecaseMock{accepted: model.ActionAccepted{
		ApplicationID: applicationID, PendingAction: "SIGN_PENDING",
	}}).Handle(withClient(t), generated.SignContractRequestObject{
		ApplicationId: common.Id(applicationID),
		Params:        generated.SignContractParams{IdempotencyKey: common.IdempotencyKey(uuid.New())},
		Body:          &generated.OfferActionRequest{OfferId: common.Id(offerID)},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitSignContractResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body generated.ActionAccepted
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusAccepted || body.ApplicationId != common.Id(applicationID) || body.PendingAction != "SIGN_PENDING" {
		t.Fatalf("status = %d, body = %+v", recorder.Code, body)
	}
}

func TestHandleRejectsMissingOffer(t *testing.T) {
	response, err := New(usecaseMock{}).Handle(withClient(t), generated.SignContractRequestObject{
		ApplicationId: common.Id(uuid.New()),
		Params:        generated.SignContractParams{IdempotencyKey: common.IdempotencyKey(uuid.New())},
		Body:          &generated.OfferActionRequest{},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitSignContractResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

type usecaseMock struct{ accepted model.ActionAccepted }

func (m usecaseMock) Execute(context.Context, model.SignContractParams) (model.ActionAccepted, error) {
	return m.accepted, nil
}

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/client/applications/1/sign", nil)
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
