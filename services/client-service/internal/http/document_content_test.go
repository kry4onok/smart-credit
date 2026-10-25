package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	documentcontent "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document/content"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	getdocumentcontent "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document/content"
	"github.com/google/uuid"
)

type contentVerifier struct {
	principal model.Principal
	err       error
}

func (v contentVerifier) Verify(_ context.Context, token string) (model.Principal, error) {
	if token != "test" {
		return model.Principal{}, errors.New("invalid signature")
	}
	return v.principal, v.err
}

type contentRepositoryFunc func(context.Context, model.GetDocumentContentParams) (model.DocumentContent, error)

func (f contentRepositoryFunc) GetDocumentContent(ctx context.Context, params model.GetDocumentContentParams) (model.DocumentContent, error) {
	return f(ctx, params)
}

func TestInternalDocumentContentHTTP(t *testing.T) {
	applicationID, documentID, userID := uuid.New(), uuid.New(), uuid.New()
	original := []byte{'%', 'P', 'D', 'F', '-', '1', '.', '4', '\n', 0, 0xff, '\r', '\n'}
	for _, tc := range []struct {
		name       string
		role       string
		header     string
		appID      string
		docID      string
		missingSub bool
		verifyErr  error
		repoErr    error
		status     int
		code       string
		called     bool
	}{
		{name: "analyst", role: "ANALYST", header: "Bearer test", status: 200, called: true},
		{name: "admin", role: "ADMIN", header: "bearer test", status: 200, called: true},
		{name: "client", role: "CLIENT", header: "Bearer test", status: 403, code: "FORBIDDEN"},
		{name: "unknown role", role: "OTHER", header: "Bearer test", status: 403, code: "FORBIDDEN"},
		{name: "no JWT", role: "ANALYST", status: 401, code: "UNAUTHORIZED"},
		{name: "wrong scheme", role: "ANALYST", header: "Basic test", status: 401, code: "UNAUTHORIZED"},
		{name: "extra token", role: "ANALYST", header: "Bearer test extra", status: 401, code: "UNAUTHORIZED"},
		{name: "wrong signature", role: "ANALYST", header: "Bearer forged", status: 401, code: "UNAUTHORIZED"},
		{name: "expired JWT", role: "ANALYST", header: "Bearer test", verifyErr: errors.New("expired"), status: 401, code: "UNAUTHORIZED"},
		{name: "missing subject", role: "ADMIN", header: "Bearer test", missingSub: true, status: 401, code: "UNAUTHORIZED"},
		{name: "zero application", role: "ANALYST", header: "Bearer test", appID: uuid.Nil.String(), status: 400, code: "INVALID_DOCUMENT_REQUEST"},
		{name: "zero document", role: "ANALYST", header: "Bearer test", docID: uuid.Nil.String(), status: 400, code: "INVALID_DOCUMENT_REQUEST"},
		{name: "malformed application", role: "ANALYST", header: "Bearer test", appID: "not-a-uuid", status: 400, code: "BAD_REQUEST"},
		{name: "malformed document", role: "ADMIN", header: "Bearer test", docID: "not-a-uuid", status: 400, code: "BAD_REQUEST"},
		{name: "not found", role: "ANALYST", header: "Bearer test", repoErr: domain.ErrDocumentNotFound, status: 404, code: "DOCUMENT_NOT_FOUND", called: true},
		{name: "storage error", role: "ADMIN", header: "Bearer test", repoErr: errors.New("read secret-storage-key: permission denied"), status: 500, code: "INTERNAL_ERROR", called: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			principal := model.Principal{UserID: userID, Role: tc.role}
			if tc.role == "CLIENT" {
				principal.OrgID = uuid.New()
			}
			if tc.missingSub {
				principal.UserID = uuid.Nil
			}
			called := false
			repo := contentRepositoryFunc(func(_ context.Context, params model.GetDocumentContentParams) (model.DocumentContent, error) {
				called = true
				if params.ApplicationID != applicationID || params.DocumentID != documentID {
					t.Fatalf("wrong document identifiers: %+v", params)
				}
				return model.DocumentContent{FileName: "Выписка.pdf", Content: original}, tc.repoErr
			})
			handler := New(Params{
				Verifier:        contentVerifier{principal: principal, err: tc.verifyErr},
				DocumentContent: documentcontent.New(getdocumentcontent.New(repo)),
			})
			appID, docID := tc.appID, tc.docID
			if appID == "" {
				appID = applicationID.String()
			}
			if docID == "" {
				docID = documentID.String()
			}
			request := httptest.NewRequest(http.MethodGet, "/internal/client/applications/"+appID+"/documents/"+docID+"/content", nil)
			request.Header.Set("Authorization", tc.header)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tc.status || called != tc.called {
				t.Fatalf("status=%d, called=%v, body=%s", recorder.Code, called, recorder.Body.String())
			}
			if recorder.Header().Get("X-Trace-ID") == "" {
				t.Fatal("missing trace ID")
			}
			if tc.status == http.StatusOK {
				if !bytes.Equal(recorder.Body.Bytes(), original) || recorder.Header().Get("Content-Type") != "application/pdf" || recorder.Header().Get("Content-Length") != strconv.Itoa(len(original)) {
					t.Fatal("original bytes or content headers changed")
				}
				disposition, params, err := mime.ParseMediaType(recorder.Header().Get("Content-Disposition"))
				if err != nil || disposition != "attachment" || params["filename"] != "Выписка.pdf" || recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatalf("download headers=%v, err=%v", recorder.Header(), err)
				}
				return
			}
			var problem common.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if problem.Code != tc.code || problem.Status != tc.status || strings.Contains(recorder.Body.String(), "secret-storage-key") || strings.Contains(recorder.Body.String(), "storage_key") {
				t.Fatalf("unexpected or unsafe problem response: %+v", problem)
			}
			if tc.status == http.StatusUnauthorized && recorder.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing authentication challenge")
			}
		})
	}
}
