package list

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

var (
	ErrInvalidLimit    = errors.New("invalid limit")
	ErrInvalidCursor   = errors.New("invalid cursor")
	ErrInvalidStatus   = errors.New("invalid status")
	ErrInvalidAssignee = errors.New("invalid assignee")
)

type UseCase struct {
	repository Repository
}

func New(repository Repository) *UseCase {
	return &UseCase{repository: repository}
}

func (uc *UseCase) Execute(ctx context.Context, input Input) (Output, error) {
	limit, err := resolveLimit(input.Limit)
	if err != nil {
		return Output{}, err
	}

	if input.Status != nil && !validStatus(*input.Status) {
		return Output{}, fmt.Errorf("%w: %q", ErrInvalidStatus, *input.Status)
	}

	after, err := decodeCursor(input.Cursor)
	if err != nil {
		return Output{}, err
	}

	assignee, err := resolveAssignee(input.Assignee, input.CurrentUserID)
	if err != nil {
		return Output{}, err
	}

	items, err := uc.repository.ListCases(ctx, RepositoryQuery{
		Limit:     limit + 1,
		After:     after,
		Status:    input.Status,
		ProductID: input.ProductID,
		Assignee:  assignee,
	})
	if err != nil {
		return Output{}, fmt.Errorf("list cases: %w", err)
	}

	output := Output{Items: items}
	if len(items) <= limit {
		return output, nil
	}

	output.Items = items[:limit]
	nextCursor, err := encodeCursor(output.Items[len(output.Items)-1])
	if err != nil {
		return Output{}, fmt.Errorf("encode next cursor: %w", err)
	}
	output.NextCursor = &nextCursor

	return output, nil
}

func resolveLimit(value *int) (int, error) {
	if value == nil {
		return defaultLimit, nil
	}
	if *value < 1 || *value > maxLimit {
		return 0, fmt.Errorf("%w: must be between 1 and %d", ErrInvalidLimit, maxLimit)
	}

	return *value, nil
}

func resolveAssignee(value *string, currentUserID uuid.UUID) (AssigneeFilter, error) {
	if value == nil {
		return AssigneeFilter{Kind: AssigneeAny}, nil
	}

	switch *value {
	case "me":
		if currentUserID == uuid.Nil {
			return AssigneeFilter{}, fmt.Errorf("%w: current user is unavailable", ErrInvalidAssignee)
		}
		return AssigneeFilter{Kind: AssigneeUser, ID: currentUserID}, nil
	case "unassigned":
		return AssigneeFilter{Kind: AssigneeUnassigned}, nil
	default:
		assigneeID, err := uuid.Parse(*value)
		if err != nil {
			return AssigneeFilter{}, fmt.Errorf("%w: expected me, unassigned or UUID", ErrInvalidAssignee)
		}
		return AssigneeFilter{Kind: AssigneeUser, ID: assigneeID}, nil
	}
}

func validStatus(status model.CaseStatus) bool {
	switch status {
	case model.CaseStatusSubmitted,
		model.CaseStatusValidation,
		model.CaseStatusNeedsInfo,
		model.CaseStatusEnrichment,
		model.CaseStatusScoring,
		model.CaseStatusDecision,
		model.CaseStatusManualReview,
		model.CaseStatusOffer,
		model.CaseStatusCounterOffer,
		model.CaseStatusOfferAccepted,
		model.CaseStatusSigning,
		model.CaseStatusDisbursed,
		model.CaseStatusRejected,
		model.CaseStatusCancelled,
		model.CaseStatusExpired:
		return true
	default:
		return false
	}
}
