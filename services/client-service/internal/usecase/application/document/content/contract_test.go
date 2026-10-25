package content

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type repositoryMock struct {
	document model.DocumentContent
	err      error
	called   bool
}

func (r *repositoryMock) GetDocumentContent(context.Context, model.GetDocumentContentParams) (model.DocumentContent, error) {
	r.called = true
	return r.document, r.err
}

func TestDocumentContentTypes(t *testing.T) {
	for _, tc := range []struct{ name, contentType string }{
		{"file.txt", "text/plain; charset=utf-8"},
		{"file.csv", "text/csv; charset=utf-8"},
		{"FILE.PDF", "application/pdf"},
		{"file.png", "image/png"},
		{"file.jpg", "image/jpeg"},
		{"file.jpeg", "image/jpeg"},
		{"file.doc", "application/msword"},
		{"file.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"file.xls", "application/vnd.ms-excel"},
		{"file.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"file.unknown", "application/octet-stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &repositoryMock{document: model.DocumentContent{FileName: tc.name, Content: []byte{0, 0xff, 10}}}
			document, err := New(repo).Execute(t.Context(), model.GetDocumentContentParams{ApplicationID: uuid.New(), DocumentID: uuid.New()})
			if err != nil || document.ContentType != tc.contentType || string(document.Content) != string(repo.document.Content) {
				t.Fatalf("content type=%s, err=%v", document.ContentType, err)
			}
		})
	}
}

func TestDocumentContentRejectsZeroIdentifiers(t *testing.T) {
	for _, params := range []model.GetDocumentContentParams{{DocumentID: uuid.New()}, {ApplicationID: uuid.New()}} {
		repo := &repositoryMock{}
		if _, err := New(repo).Execute(t.Context(), params); !errors.Is(err, domain.ErrDocumentNotFound) || repo.called {
			t.Fatalf("zero identifier: called=%v, err=%v", repo.called, err)
		}
	}
}

func TestDocumentContentPreservesRepositoryError(t *testing.T) {
	repo := &repositoryMock{err: domain.ErrDocumentNotFound}
	if _, err := New(repo).Execute(t.Context(), model.GetDocumentContentParams{ApplicationID: uuid.New(), DocumentID: uuid.New()}); !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("err=%v", err)
	}
}
