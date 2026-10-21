package list

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

func TestUseCaseExecuteBuildsNextPageCursor(t *testing.T) {
	createdAt := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)
	items := make([]model.CaseListItem, defaultLimit+1)
	for index := range items {
		items[index] = model.CaseListItem{
			ID:        uuid.New(),
			Status:    model.CaseStatusManualReview,
			CreatedAt: createdAt.Add(time.Duration(index) * time.Minute),
		}
	}

	var capturedQuery RepositoryQuery
	uc := New(repositoryMock{listCases: func(
		_ context.Context,
		query RepositoryQuery,
	) ([]model.CaseListItem, error) {
		capturedQuery = query
		return items, nil
	}})

	output, err := uc.Execute(context.Background(), Input{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if capturedQuery.Limit != defaultLimit+1 {
		t.Fatalf("repository limit = %d, want %d", capturedQuery.Limit, defaultLimit+1)
	}
	if len(output.Items) != defaultLimit {
		t.Fatalf("items count = %d, want %d", len(output.Items), defaultLimit)
	}
	if output.NextCursor == nil {
		t.Fatal("next cursor is nil")
	}

	cursor, err := decodeCursor(output.NextCursor)
	if err != nil {
		t.Fatalf("decodeCursor() error = %v", err)
	}
	lastItem := output.Items[len(output.Items)-1]
	if cursor.ID != lastItem.ID || !cursor.CreatedAt.Equal(lastItem.CreatedAt) || !cursor.ManualReview {
		t.Fatalf("cursor = %+v, want last item %+v", cursor, lastItem)
	}
}

func TestUseCaseExecuteResolvesMeAssignee(t *testing.T) {
	currentUserID := uuid.New()
	assignee := "me"

	uc := New(repositoryMock{listCases: func(
		_ context.Context,
		query RepositoryQuery,
	) ([]model.CaseListItem, error) {
		if query.Assignee.Kind != AssigneeUser || query.Assignee.ID != currentUserID {
			t.Fatalf("assignee = %+v, want current user %s", query.Assignee, currentUserID)
		}
		return nil, nil
	}})

	_, err := uc.Execute(context.Background(), Input{
		Assignee:      &assignee,
		CurrentUserID: currentUserID,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestUseCaseExecuteRejectsInvalidInput(t *testing.T) {
	invalidLimit := maxLimit + 1
	invalidCursor := "not-base64"
	invalidStatus := model.CaseStatus("UNKNOWN")
	invalidAssignee := "somebody"

	tests := []struct {
		name  string
		input Input
		want  error
	}{
		{name: "limit", input: Input{Limit: &invalidLimit}, want: ErrInvalidLimit},
		{name: "cursor", input: Input{Cursor: &invalidCursor}, want: ErrInvalidCursor},
		{name: "status", input: Input{Status: &invalidStatus}, want: ErrInvalidStatus},
		{name: "assignee", input: Input{Assignee: &invalidAssignee}, want: ErrInvalidAssignee},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := New(repositoryMock{listCases: func(
				context.Context,
				RepositoryQuery,
			) ([]model.CaseListItem, error) {
				t.Fatal("repository must not be called")
				return nil, nil
			}})

			_, err := uc.Execute(context.Background(), tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Execute() error = %v, want %v", err, tt.want)
			}
		})
	}
}
