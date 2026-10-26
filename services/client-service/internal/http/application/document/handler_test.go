package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

func TestHandleUploadsDocument(t *testing.T) {
	applicationID := uuid.New()
	userID := uuid.New()
	orgID := uuid.New()
	mock := &usecaseMock{document: model.ApplicationDocument{
		ID: uuid.New(), DocumentType: "BANK_STATEMENT", FileName: "statement.pdf",
		Sha256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", IsMock: true,
		UploadedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
	}}
	body := multipartBody(t, "BANK_STATEMENT", "statement.pdf", []byte("demo"))
	response, err := New(mock).Handle(withClient(t, userID, orgID), generated.AddApplicationDocumentRequestObject{
		ApplicationId: common.Id(applicationID),
		Params:        generated.AddApplicationDocumentParams{IdempotencyKey: uuid.New()},
		Body:          body,
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitAddApplicationDocumentResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusCreated || !mock.called || mock.params.ApplicationID != applicationID || mock.params.UserID != userID || mock.params.OrgID != orgID || mock.params.DocumentType != "BANK_STATEMENT" || mock.params.FileName != "statement.pdf" || string(mock.params.Content) != "demo" {
		t.Fatalf("status = %d, called = %v, params = %+v", recorder.Code, mock.called, mock.params)
	}
	var metadata common.DocumentMetadata
	if err := json.Unmarshal(recorder.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Id != common.Id(mock.document.ID) || metadata.FileName != mock.document.FileName || metadata.Sha256 != mock.document.Sha256 {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestHandleRejectsInvalidMultipart(t *testing.T) {
	mock := &usecaseMock{}
	body := multipartBody(t, "BANK_STATEMENT", "statement.pdf", bytes.Repeat([]byte{'x'}, 5*1024*1024+1))
	response, err := New(mock).Handle(withClient(t, uuid.New(), uuid.New()), generated.AddApplicationDocumentRequestObject{
		ApplicationId: uuid.New(), Params: generated.AddApplicationDocumentParams{IdempotencyKey: uuid.New()}, Body: body,
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitAddApplicationDocumentResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusUnprocessableEntity || mock.called {
		t.Fatalf("status = %d, called = %v", recorder.Code, mock.called)
	}
}

func TestHandleMapsRepositoryError(t *testing.T) {
	mock := &usecaseMock{err: errors.New("unexpected")}
	response, err := New(mock).Handle(withClient(t, uuid.New(), uuid.New()), generated.AddApplicationDocumentRequestObject{
		ApplicationId: uuid.New(), Params: generated.AddApplicationDocumentParams{IdempotencyKey: uuid.New()}, Body: multipartBody(t, "BANK_STATEMENT", "statement.pdf", []byte("demo")),
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitAddApplicationDocumentResponse(recorder); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func multipartBody(t *testing.T, documentType, fileName string, content []byte) *multipart.Reader {
	t.Helper()
	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("document_type", documentType); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(body, writer.Boundary())
	return reader
}

func withClient(t *testing.T, userID, orgID uuid.UUID) context.Context {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/client/applications/1/documents", nil)
	request.Header.Set("Authorization", "Bearer test")
	var captured context.Context
	middleware.RequireClient(verifierMock{principal: model.Principal{UserID: userID, OrgID: orgID, Role: "CLIENT"}}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
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

func TestMultipartRequirementMetadata(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for name, value := range map[string]string{"document_type": "PATENT", "requirement_id": "patent_right:2026-10-26:2026-10-26:psn", "coverage_from": "2026-01-01", "coverage_to": "2026-12-31"} {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
		if duplicate {
			if err := writer.WriteField("coverage_from", "2026-02-01"); err != nil {
				t.Fatal(err)
			}
		}
		part, err := writer.CreateFormFile("file", "patent.pdf")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("content")); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		result, err := parseMultipart(multipart.NewReader(&body, writer.Boundary()))
		if duplicate {
			if err == nil {
				t.Fatal("duplicate metadata accepted")
			}
			continue
		}
		if err != nil || result.RequirementID == nil || result.CoverageFrom == nil || result.CoverageTo == nil || result.CoverageFrom.Format(time.DateOnly) != "2026-01-01" {
			t.Fatalf("upload=%+v err=%v", result, err)
		}
	}
}
