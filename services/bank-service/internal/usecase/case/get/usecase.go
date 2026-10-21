package get

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("case not found")

type UseCase struct {
	repository Repository
}

func New(repository Repository) *UseCase {
	return &UseCase{repository: repository}
}

func (uc *UseCase) Execute(ctx context.Context, input Input) (Output, error) {
	caseItem, found, err := uc.repository.GetCase(ctx, input.CaseID)
	if err != nil {
		return Output{}, fmt.Errorf("get case: %w", err)
	}
	if !found {
		return Output{}, ErrNotFound
	}

	return Output{
		Case:           caseItem,
		AllowedActions: allowedActions(caseItem, input.CurrentUserID),
	}, nil
}

func allowedActions(caseItem model.Case, currentUserID uuid.UUID) []model.CaseAction {
	actions := []model.CaseAction{model.CaseActionView}
	if caseItem.Status != model.CaseStatusManualReview || currentUserID == uuid.Nil {
		return actions
	}

	if caseItem.AssigneeID == nil {
		return append(actions, model.CaseActionTake)
	}
	if *caseItem.AssigneeID == currentUserID {
		return append(actions, model.CaseActionDecide)
	}

	return actions
}
