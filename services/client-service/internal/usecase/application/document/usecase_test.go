package document

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type repositoryMock struct {
	called bool
	params model.AddApplicationDocumentParams
}

func (m *repositoryMock) AddDocument(_ context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
	m.called = true
	m.params = params
	return model.ApplicationDocument{DocumentType: params.DocumentType}, nil
}

func validParams() model.AddApplicationDocumentParams {
	return model.AddApplicationDocumentParams{
		OrgID: uuid.New(), UserID: uuid.New(), ApplicationID: uuid.New(), IdempotencyKey: uuid.New(),
		DocumentType: "BANK_STATEMENT", FileName: "statement.txt", Content: []byte("demo"),
	}
}

func TestExecutePassesValidDocument(t *testing.T) {
	repository := &repositoryMock{}
	if _, err := New(repository).Execute(context.Background(), validParams()); err != nil {
		t.Fatal(err)
	}
	if !repository.called {
		t.Fatal("repository was not called")
	}
}

func TestExecuteRejectsInvalidDocument(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*model.AddApplicationDocumentParams)
	}{
		{name: "type", mutate: func(params *model.AddApplicationDocumentParams) { params.DocumentType = "OTHER" }},
		{name: "filename", mutate: func(params *model.AddApplicationDocumentParams) { params.FileName = " " }},
		{name: "size", mutate: func(params *model.AddApplicationDocumentParams) { params.Content = make([]byte, MaxDocumentSize+1) }},
		{name: "empty", mutate: func(params *model.AddApplicationDocumentParams) { params.Content = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			params := validParams()
			test.mutate(&params)
			repository := &repositoryMock{}
			_, err := New(repository).Execute(context.Background(), params)
			if !errors.Is(err, domain.ErrInvalidDocumentUpload) || repository.called {
				t.Fatalf("error = %v, called = %v", err, repository.called)
			}
		})
	}
}
