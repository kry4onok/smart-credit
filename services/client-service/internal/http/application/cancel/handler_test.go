package cancel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	cancelapplication "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/cancel"
	"github.com/google/uuid"
)

type usecaseMock struct {
	params model.CancelApplicationParams
	err    error
	called bool
}

func (m *usecaseMock) Execute(_ context.Context, params model.CancelApplicationParams) (model.ActionAccepted, error) {
	m.called, m.params = true, params
	return model.ActionAccepted{ApplicationID: params.ApplicationID, PendingAction: "CANCEL_PENDING"}, m.err
}

type verifierMock struct{ principal model.Principal }

func (m verifierMock) Verify(context.Context, string) (model.Principal, error) {
	return m.principal, nil
}

func clientContext(t *testing.T, principal model.Principal) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer test")
	var ctx context.Context
	middleware.RequireClient(verifierMock{principal}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })).ServeHTTP(httptest.NewRecorder(), request)
	if ctx == nil {
		t.Fatal("client context not captured")
	}
	return ctx
}

func TestHandleCancellation(t *testing.T) {
	principal := model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}
	ctx := clientContext(t, principal)
	applicationID, key := uuid.New(), uuid.New()
	reason := "Нашли другой источник финансирования"
	var emptyHash []byte
	for _, tc := range []struct {
		name string
		body *generated.CancelApplicationJSONRequestBody
		want string
	}{
		{"no body", nil, ""},
		{"empty object", &generated.CancelApplicationJSONRequestBody{}, ""},
		{"with reason", &generated.CancelApplicationJSONRequestBody{Reason: &reason}, reason},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &usecaseMock{}
			response, err := New(usecase).Handle(ctx, generated.CancelApplicationRequestObject{
				ApplicationId: applicationID, Params: generated.CancelApplicationParams{IdempotencyKey: key}, Body: tc.body,
			})
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if err := response.VisitCancelApplicationResponse(recorder); err != nil {
				t.Fatal(err)
			}
			var accepted generated.ActionAccepted
			if err := json.Unmarshal(recorder.Body.Bytes(), &accepted); err != nil {
				t.Fatal(err)
			}
			params := usecase.params
			if recorder.Code != http.StatusAccepted || accepted.ApplicationId != applicationID || accepted.PendingAction != "CANCEL_PENDING" ||
				params.OrgID != principal.OrgID || params.UserID != principal.UserID || params.ApplicationID != applicationID || params.IdempotencyID != key || params.Reason != tc.want || len(params.RequestHash) != 32 {
				t.Fatalf("status=%d, response=%+v, params=%+v", recorder.Code, accepted, params)
			}
			if tc.name == "no body" {
				emptyHash = params.RequestHash
			} else if tc.name == "empty object" && !bytes.Equal(emptyHash, params.RequestHash) {
				t.Fatal("no body and empty object must share a request hash")
			} else if tc.name == "with reason" && bytes.Equal(emptyHash, params.RequestHash) {
				t.Fatal("changed reason must change the request hash")
			}
		})
	}
}

func TestHandleCancellationErrors(t *testing.T) {
	principal := model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}
	ctx := clientContext(t, principal)
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		id     uuid.UUID
		key    uuid.UUID
		err    error
		status int
		code   string
		called bool
	}{
		{"unauthorized", context.Background(), uuid.New(), uuid.New(), nil, 401, "UNAUTHORIZED", false},
		{"zero application", ctx, uuid.Nil, uuid.New(), nil, 400, "INVALID_CANCEL_ACTION", false},
		{"zero key", ctx, uuid.New(), uuid.Nil, nil, 400, "INVALID_CANCEL_ACTION", false},
		{"not found", ctx, uuid.New(), uuid.New(), domain.ErrApplicationNotFound, 404, "APPLICATION_NOT_FOUND", true},
		{"state conflict", ctx, uuid.New(), uuid.New(), domain.ErrApplicationCancelConflict, 409, "CANCEL_ACTION_CONFLICT", true},
		{"key conflict", ctx, uuid.New(), uuid.New(), domain.ErrIdempotencyConflict, 409, "IDEMPOTENCY_CONFLICT", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			usecase := &usecaseMock{err: tc.err}
			response, err := New(usecase).Handle(tc.ctx, generated.CancelApplicationRequestObject{ApplicationId: tc.id, Params: generated.CancelApplicationParams{IdempotencyKey: tc.key}})
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if err := response.VisitCancelApplicationResponse(recorder); err != nil {
				t.Fatal(err)
			}
			var problem common.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || problem.Code != tc.code || usecase.called != tc.called {
				t.Fatalf("status=%d, problem=%+v, called=%v", recorder.Code, problem, usecase.called)
			}
		})
	}
}

func TestHandleRejectsLongCancellationReason(t *testing.T) {
	repo := &usecaseMock{}
	reason := strings.Repeat("я", 501)
	principal := model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}
	response, err := New(cancelapplication.New(cancelRepositoryAdapter{repo})).Handle(clientContext(t, principal), generated.CancelApplicationRequestObject{
		ApplicationId: uuid.New(), Params: generated.CancelApplicationParams{IdempotencyKey: uuid.New()}, Body: &generated.CancelApplicationJSONRequestBody{Reason: &reason},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitCancelApplicationResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnprocessableEntity || repo.called {
		t.Fatalf("status=%d, repository called=%v", recorder.Code, repo.called)
	}
}

type cancelRepositoryAdapter struct{ mock *usecaseMock }

func (a cancelRepositoryAdapter) CancelApplication(ctx context.Context, params model.CancelApplicationParams) (model.ActionAccepted, error) {
	return a.mock.Execute(ctx, params)
}
