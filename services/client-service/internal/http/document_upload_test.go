package httpapi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	applicationdocument "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/application/document"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type uploadUsecaseFunc func(context.Context, model.AddApplicationDocumentParams) (model.ApplicationDocument, error)

func (f uploadUsecaseFunc) Execute(ctx context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
	return f(ctx, params)
}

func TestMultipartDocumentHTTP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content []byte
		status  int
	}{
		{"binary attachment", []byte{0, 1, 0xff, 10, 42}, 201},
		{"empty attachment", nil, 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userID, orgID, appID := uuid.New(), uuid.New(), uuid.New()
			called := false
			handler := New(Params{Verifier: verifierMock{principal: model.Principal{UserID: userID, OrgID: orgID, Role: "CLIENT"}},
				ApplicationDocument: applicationdocument.New(uploadUsecaseFunc(func(_ context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
					called = true
					if params.OrgID != orgID || params.UserID != userID || params.ApplicationID != appID || params.DocumentType != "BANK_STATEMENT" || params.FileName != "Выписка.pdf" || !bytes.Equal(params.Content, tc.content) {
						t.Fatal("multipart parsing altered uploaded bytes or metadata")
					}
					return model.ApplicationDocument{ID: uuid.New(), DocumentType: params.DocumentType, FileName: params.FileName, IsMock: true}, nil
				})),
			})
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if err := writer.WriteField("document_type", "BANK_STATEMENT"); err != nil {
				t.Fatal(err)
			}
			part, err := writer.CreateFormFile("file", "Выписка.pdf")
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
		})
	}
}
