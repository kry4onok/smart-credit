package list

import (
	"context"
	"errors"
	"fmt"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

var (
	ErrInvalidLimit  = errors.New("invalid limit")
	ErrInvalidCursor = errors.New("invalid cursor")
	ErrInvalidStatus = errors.New("invalid status")
)

func (u *Usecase) Execute(ctx context.Context, input Input) (Output, error) {
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
	items, err := u.repository.List(ctx, RepositoryQuery{
		OrgID: input.OrgID, Limit: limit + 1, After: after, Status: input.Status,
	})
	if err != nil {
		return Output{}, fmt.Errorf("list applications: %w", err)
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

func validStatus(status string) bool {
	switch status {
	case "DRAFT", "SUBMITTED", "UNDER_REVIEW", "NEEDS_INFO", "OFFER_AVAILABLE", "FORMALIZING", "DISBURSED", "REJECTED", "CLOSED":
		return true
	default:
		return false
	}
}
