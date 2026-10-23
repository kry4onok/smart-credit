package list

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	listapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/list"
	"github.com/google/uuid"
)

func TestHandleListsApplications(t *testing.T) {
	orgID := uuid.New()
	status := common.ClientStatusDRAFT
	limit := 10
	cursor := "opaque"
	amount := int64(12500)
	nextCursor := "next"
	mock := &usecaseMock{output: listapplication.Output{
		Items: []model.ApplicationListItem{{
			ID: uuid.New(), ProductID: uuid.New(), AmountKopecks: &amount,
			Status: "DRAFT", NextAction: "Заполните заявку", CreatedAt: time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC),
		}},
		NextCursor: &nextCursor,
	}}
	response, err := New(mock).Handle(withClient(t, orgID), generated.ListApplicationsRequestObject{
		Params: generated.ListApplicationsParams{Cursor: &cursor, Limit: &limit, Status: &status},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitListApplicationsResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || mock.input.OrgID != orgID || mock.input.Limit == nil || *mock.input.Limit != limit || mock.input.Cursor == nil || mock.input.Status == nil {
		t.Fatalf("status = %d, input = %+v", recorder.Code, mock.input)
	}
	var body generated.ListApplications200JSONResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Amount == nil || body.Items[0].Amount.AmountKopecks != amount || body.NextCursor == nil || *body.NextCursor != nextCursor {
		t.Fatalf("body = %+v", body)
	}
}

func TestHandleRequiresClient(t *testing.T) {
	mock := &usecaseMock{}
	response, err := New(mock).Handle(context.Background(), generated.ListApplicationsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitListApplicationsResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnauthorized || mock.input.OrgID != uuid.Nil {
		t.Fatalf("status = %d, input = %+v", recorder.Code, mock.input)
	}
}

func withClient(t *testing.T, orgID uuid.UUID) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/client/applications", nil)
	request.Header.Set("Authorization", "Bearer test")
	var captured context.Context
	middleware.RequireClient(verifierMock{principal: model.Principal{UserID: uuid.New(), OrgID: orgID, Role: "CLIENT"}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
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
