package delete

import (
	"context"
	"encoding/json"
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

type usecaseMock struct {
	params model.DeleteApplicationParams
	called bool
	err    error
}

func (m *usecaseMock) Execute(_ context.Context, p model.DeleteApplicationParams) error {
	m.called = true
	m.params = p
	return m.err
}
func TestDeleteProtocol(t *testing.T) {
	principal := model.Principal{UserID: uuid.New(), OrgID: uuid.New(), Role: "CLIENT"}
	ctx := clientContext(t, principal)
	id := uuid.New()
	for _, tc := range []struct {
		name, etag string
		ctx        context.Context
		err        error
		status     int
		code       string
		called     bool
	}{
		{"success", `"3"`, ctx, nil, 204, "", true},
		{"unauthenticated", `"3"`, context.Background(), nil, 401, "UNAUTHORIZED", false},
		{"missing version", "", ctx, nil, 400, "INVALID_IF_MATCH", false},
		{"weak version", `W/"3"`, ctx, nil, 400, "INVALID_IF_MATCH", false},
		{"wildcard", "*", ctx, nil, 400, "INVALID_IF_MATCH", false},
		{"stale version", `"3"`, ctx, domain.ErrApplicationVersionConflict, 409, "APPLICATION_VERSION_CONFLICT", true},
		{"submitted", `"3"`, ctx, domain.ErrApplicationNotDraft, 409, "APPLICATION_STATE_CONFLICT", true},
		{"missing or foreign", `"3"`, ctx, domain.ErrApplicationNotFound, 404, "APPLICATION_NOT_FOUND", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &usecaseMock{err: tc.err}
			response, err := New(m).Handle(tc.ctx, generated.DeleteApplicationRequestObject{ApplicationId: id, Params: generated.DeleteApplicationParams{IfMatch: tc.etag}})
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if err = response.VisitDeleteApplicationResponse(recorder); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.status || m.called != tc.called {
				t.Fatalf("status=%d called=%v", recorder.Code, m.called)
			}
			if tc.called && (m.params.OrgID != principal.OrgID || m.params.ApplicationID != id || m.params.ExpectedVersion != 3) {
				t.Fatalf("params=%+v", m.params)
			}
			if tc.status == 204 {
				if recorder.Body.Len() != 0 {
					t.Fatal("204 must have empty body")
				}
			} else {
				var p common.Problem
				if err = json.Unmarshal(recorder.Body.Bytes(), &p); err != nil {
					t.Fatal(err)
				}
				if p.Code != tc.code {
					t.Fatalf("code=%s", p.Code)
				}
			}
		})
	}
}
