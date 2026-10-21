package get

import (
	"context"
	"errors"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

func TestUseCaseExecuteReturnsNotFound(t *testing.T) {
	uc := New(repositoryMock{getCase: func(
		context.Context,
		uuid.UUID,
	) (model.Case, bool, error) {
		return model.Case{}, false, nil
	}})

	_, err := uc.Execute(context.Background(), Input{CaseID: uuid.New()})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, ErrNotFound)
	}
}

func TestUseCaseExecuteBuildsAllowedActions(t *testing.T) {
	currentUserID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name       string
		caseItem   model.Case
		userID     uuid.UUID
		wantAction model.CaseAction
	}{
		{
			name:       "unassigned manual review can be taken",
			caseItem:   model.Case{Status: model.CaseStatusManualReview},
			userID:     currentUserID,
			wantAction: model.CaseActionTake,
		},
		{
			name: "assigned manual review can be decided",
			caseItem: model.Case{
				Status:     model.CaseStatusManualReview,
				AssigneeID: &currentUserID,
			},
			userID:     currentUserID,
			wantAction: model.CaseActionDecide,
		},
		{
			name: "another analyst can only view",
			caseItem: model.Case{
				Status:     model.CaseStatusManualReview,
				AssigneeID: &otherUserID,
			},
			userID:     currentUserID,
			wantAction: model.CaseActionView,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := New(repositoryMock{getCase: func(
				context.Context,
				uuid.UUID,
			) (model.Case, bool, error) {
				return tt.caseItem, true, nil
			}})

			output, err := uc.Execute(context.Background(), Input{
				CaseID:        uuid.New(),
				CurrentUserID: tt.userID,
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if output.AllowedActions[len(output.AllowedActions)-1] != tt.wantAction {
				t.Fatalf("actions = %v, want last action %q", output.AllowedActions, tt.wantAction)
			}
		})
	}
}
