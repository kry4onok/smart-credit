package get

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

func TestHandleReturnsApplicationDetails(t *testing.T) {
	applicationID := uuid.New()
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	usecase := &usecaseMock{application: model.Application{
		ID: applicationID, ProductID: uuid.New(), ProductVersion: 3,
		Company: model.Company{INN: "7707083893", Name: "Demo", OKVED: "62.01", RegisteredAt: createdAt},
		Form:    &model.ApplicationForm{AmountKopecks: 100_00, TermMonths: 12, Purpose: "EQUIPMENT"},
		Status:  "OFFER_AVAILABLE", StageLabel: "Предложение", NextAction: "Выберите предложение",
		AllowedActions: []string{"ACCEPT_OFFER", "DECLINE_OFFER"},
		Documents:      []model.ApplicationDocument{{ID: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "statement.pdf", Sha256: "abc", IsMock: true, UploadedAt: createdAt}},
		InfoRequest:    &model.ApplicationInfoRequest{ID: uuid.New(), Message: "Нужна выписка", RequiredFields: []string{"COMPANY_REVENUE_12M"}, RequiredDocumentTypes: []string{"BANK_STATEMENT"}, CreatedAt: createdAt},
		Offer:          &model.ApplicationOffer{ID: uuid.New(), Kind: "OFFER", ProductID: uuid.New(), ProductVersion: 4, ProductName: "Экспресс", AmountKopecks: 500_00, TermMonths: 6, RateBps: 1900, Explanation: "Подходит", ExpiresAt: createdAt.Add(24 * time.Hour)},
		PublicReason:   stringPtr("Предварительное решение"), Version: 7, CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Hour),
	}}
	applicationID = usecase.application.ID
	response, err := New(usecase).Handle(withClient(t), generated.GetApplicationRequestObject{ApplicationId: common.Id(applicationID)})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body generated.ClientApplication
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || recorder.Header().Get("ETag") != `"7"` || !usecase.called || body.Id != common.Id(applicationID) || body.Form == nil || body.Form.Amount.AmountKopecks != 100_00 || body.Offer == nil || len(body.Documents) != 1 || body.InfoRequest == nil {
		t.Fatalf("status = %d, headers = %v, called = %v, body = %+v", recorder.Code, recorder.Header(), usecase.called, body)
	}
}

func TestHandleMapsNotFound(t *testing.T) {
	usecase := &usecaseMock{err: domain.ErrApplicationNotFound}
	response, err := New(usecase).Handle(withClient(t), generated.GetApplicationRequestObject{ApplicationId: common.Id(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var problem common.Problem
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusNotFound || problem.Code != "APPLICATION_NOT_FOUND" {
		t.Fatalf("status = %d, problem = %+v", recorder.Code, problem)
	}
}

func TestHandleRejectsMissingClient(t *testing.T) {
	response, err := New(&usecaseMock{}).Handle(context.Background(), generated.GetApplicationRequestObject{ApplicationId: common.Id(uuid.New())})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func withClient(t *testing.T) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/client/applications/1", nil)
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

func stringPtr(value string) *string { return &value }
