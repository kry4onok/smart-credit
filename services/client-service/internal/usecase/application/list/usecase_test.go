package list

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type repositoryMock struct {
	items []model.ApplicationListItem
	query RepositoryQuery
}

func (m *repositoryMock) List(_ context.Context, query RepositoryQuery) ([]model.ApplicationListItem, error) {
	m.query = query
	return m.items, nil
}

func TestExecutePaginatesWithCursor(t *testing.T) {
	first := model.ApplicationListItem{ID: uuid.New(), CreatedAt: time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)}
	second := model.ApplicationListItem{ID: uuid.New(), CreatedAt: first.CreatedAt.Add(time.Minute)}
	repository := &repositoryMock{items: []model.ApplicationListItem{first, second}}
	limit := 1
	output, err := New(repository).Execute(context.Background(), Input{Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Items) != 1 || output.NextCursor == nil {
		t.Fatalf("output = %+v", output)
	}
	decoded, err := decodeCursor(output.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID != first.ID || !decoded.CreatedAt.Equal(first.CreatedAt) || repository.query.Limit != 2 {
		t.Fatalf("cursor = %+v, query = %+v", decoded, repository.query)
	}
}

func TestExecuteRejectsInvalidFilters(t *testing.T) {
	for _, test := range []struct {
		name  string
		input Input
		want  error
	}{
		{name: "limit", input: Input{Limit: func() *int { value := 101; return &value }()}, want: ErrInvalidLimit},
		{name: "cursor", input: Input{Cursor: func() *string { value := "bad"; return &value }()}, want: ErrInvalidCursor},
		{name: "status", input: Input{Status: func() *string { value := "UNKNOWN"; return &value }()}, want: ErrInvalidStatus},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(&repositoryMock{}).Execute(context.Background(), test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}
