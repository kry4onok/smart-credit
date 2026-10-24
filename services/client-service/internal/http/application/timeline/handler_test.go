package timeline

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type usecaseMock struct {
	entries              []model.TimelineEntry
	err                  error
	orgID, applicationID uuid.UUID
	called               bool
}

func (m *usecaseMock) Execute(_ context.Context, orgID, applicationID uuid.UUID) ([]model.TimelineEntry, error) {
	m.called, m.orgID, m.applicationID = true, orgID, applicationID
	return m.entries, m.err
}

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

func clientContext(t *testing.T, principal model.Principal) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer test")
	var ctx context.Context
	middleware.RequireClient(verifierMock{principal}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), request)
	if ctx == nil {
		t.Fatal("client context was not captured")
	}
	return ctx
}

func TestHandleReturnsOnlyPublicTimelineFields(t *testing.T) {
	principal := model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}
	applicationID, entryID := uuid.New(), uuid.New()
	occurredAt := time.Date(2026, 10, 24, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	usecase := &usecaseMock{entries: []model.TimelineEntry{
		{ID: entryID, Status: "SUBMITTED", Message: "Заявка отправлена в банк", OccurredAt: occurredAt},
	}}
	response, err := New(usecase).Handle(clientContext(t, principal), generated.GetApplicationTimelineRequestObject{ApplicationId: applicationID})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetApplicationTimelineResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{{
		"id": entryID.String(), "status": "SUBMITTED", "message": "Заявка отправлена в банк", "occurred_at": "2026-10-24T09:00:00Z",
	}}
	if recorder.Code != http.StatusOK || recorder.Header().Get("Cache-Control") != "no-store" || !reflect.DeepEqual(body.Items, want) {
		t.Fatalf("status=%d, headers=%v, items=%v", recorder.Code, recorder.Header(), body.Items)
	}
	if !usecase.called || usecase.orgID != principal.OrgID || usecase.applicationID != applicationID {
		t.Fatalf("wrong usecase arguments: %+v", usecase)
	}
}

func TestHandleReturnsEmptyArray(t *testing.T) {
	principal := model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}
	response, err := New(&usecaseMock{}).Handle(clientContext(t, principal), generated.GetApplicationTimelineRequestObject{ApplicationId: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetApplicationTimelineResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || recorder.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("status=%d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleErrors(t *testing.T) {
	principal := model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		err    error
		status int
		code   string
		called bool
	}{
		{"missing client", context.Background(), nil, http.StatusUnauthorized, "UNAUTHORIZED", false},
		{"not found", clientContext(t, principal), domain.ErrApplicationNotFound, http.StatusNotFound, "APPLICATION_NOT_FOUND", true},
		{"database failure", clientContext(t, principal), errors.New("private database details"), http.StatusInternalServerError, "INTERNAL_ERROR", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &usecaseMock{err: tc.err}
			response, err := New(usecase).Handle(tc.ctx, generated.GetApplicationTimelineRequestObject{ApplicationId: uuid.New()})
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if err := response.VisitGetApplicationTimelineResponse(recorder); err != nil {
				t.Fatal(err)
			}
			var problem common.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || problem.Code != tc.code || problem.TraceId == "" || usecase.called != tc.called || recorder.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("status=%d, problem=%+v, called=%v", recorder.Code, problem, usecase.called)
			}
		})
	}
}

func TestTimelineRejectsNonClientRole(t *testing.T) {
	for _, role := range []string{"ANALYST", "ADMIN"} {
		t.Run(role, func(t *testing.T) {
			called := false
			protected := middleware.RequireClient(verifierMock{model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: role}}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))
			request := httptest.NewRequest(http.MethodGet, "/client/applications/"+uuid.NewString()+"/timeline", nil)
			request.Header.Set("Authorization", "Bearer test")
			recorder := httptest.NewRecorder()
			protected.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden || called {
				t.Fatalf("status=%d, called=%v", recorder.Code, called)
			}
		})
	}
}
