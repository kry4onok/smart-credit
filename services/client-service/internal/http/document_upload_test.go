package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/api/common"
	applicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	documentusecase "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/usecase/application/document"
	"github.com/google/uuid"
)

type uploadRepositoryFunc func(context.Context, model.AddApplicationDocumentParams) (model.ApplicationDocument, error)

func (f uploadRepositoryFunc) AddDocument(ctx context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
	return f(ctx, params)
}

func TestMultipartDocumentHTTP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		file    string
		content []byte
		status  int
	}{
		{"UTF-8 attachment", "Выписка.txt", []byte("Демо-выписка\r\nОстаток: 1000.00"), 201},
		{"empty attachment", "Выписка.txt", nil, 422},
		{"oversized attachment", "Выписка.txt", bytes.Repeat([]byte{'a'}, documentusecase.MaxDocumentSize+1), 422},
		{"renamed text as PDF", "Выписка.pdf", []byte("not a PDF"), 422},
		{"unsupported extension", "Выписка.exe", []byte("MZ executable"), 422},
		{"binary attachment as text", "Выписка.txt", []byte{0, 1, 0xff, 10, 42}, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID, orgID, appID := uuid.New(), uuid.New(), uuid.New()
			called := false
			handler := New(Params{Verifier: verifierMock{principal: model.Principal{UserID: userID, OrgID: orgID, Role: "CLIENT"}},
				ApplicationDocument: applicationdocument.New(documentusecase.New(uploadRepositoryFunc(func(_ context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
					called = true
					if params.OrgID != orgID || params.UserID != userID || params.ApplicationID != appID || params.DocumentType != "BANK_STATEMENT" || params.FileName != tc.file || !bytes.Equal(params.Content, tc.content) {
						t.Fatal("multipart parsing altered uploaded bytes or metadata")
					}
					return model.ApplicationDocument{ID: uuid.New(), DocumentType: params.DocumentType, FileName: params.FileName, IsMock: true}, nil
				}))),
			})
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if err := writer.WriteField("document_type", "BANK_STATEMENT"); err != nil {
				t.Fatal(err)
			}
			part, err := writer.CreateFormFile("file", tc.file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(tc.content); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodPost, "/client/applications/"+appID.String()+"/documents", &body)
			r.Header.Set("Authorization", "Bearer test")
			r.Header.Set("Idempotency-Key", uuid.NewString())
			r.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status || called != (tc.status == 201) {
				t.Fatalf("status=%d, usecase called=%v", w.Code, called)
			}
			if tc.status == http.StatusUnprocessableEntity {
				var problem common.Problem
				if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != "INVALID_DOCUMENT" || problem.Detail == nil || *problem.Detail == "" {
					t.Fatalf("upload error lost code or detail: %+v", problem)
				}
			}
		})
	}
}
