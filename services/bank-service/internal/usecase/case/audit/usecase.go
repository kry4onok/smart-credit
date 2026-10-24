package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

var (
	ErrNotFound      = errors.New("case not found")
	ErrInvalidCursor = errors.New("invalid audit cursor")
	ErrInvalidLimit  = errors.New("invalid audit limit")
)

type Entry struct {
	ID                   uuid.UUID
	Action, ActorType    string
	ActorID              *uuid.UUID
	FromStatus, ToStatus *string
	Reason               string
	OccurredAt           time.Time
}

type Cursor struct {
	OccurredAt time.Time
	ID         uuid.UUID
}
type Query struct {
	CaseID uuid.UUID
	After  *Cursor
	Limit  int
}

type Input struct {
	CaseID uuid.UUID
	Cursor *string
	Limit  *int
}

type Output struct {
	Items      []Entry
	NextCursor *string
}

type Repository interface {
	CaseExists(context.Context, uuid.UUID) (bool, error)
	ListAudit(context.Context, Query) ([]Entry, error)
}

type UseCase struct{ repository Repository }

func New(r Repository) *UseCase { return &UseCase{r} }

func (u *UseCase) Execute(ctx context.Context, in Input) (Output, error) {
	limit := DefaultLimit
	if in.Limit != nil {
		limit = *in.Limit
	}
	if limit < 1 || limit > MaxLimit {
		return Output{}, ErrInvalidLimit
	}
	after, err := decodeCursor(in.CaseID, in.Cursor)
	if err != nil {
		return Output{}, err
	}
	exists, err := u.repository.CaseExists(ctx, in.CaseID)
	if err != nil {
		return Output{}, fmt.Errorf("check audit case: %w", err)
	}
	if !exists {
		return Output{}, ErrNotFound
	}
	items, err := u.repository.ListAudit(ctx, Query{CaseID: in.CaseID, After: after, Limit: limit + 1})
	if err != nil {
		return Output{}, fmt.Errorf("list case audit: %w", err)
	}
	out := Output{Items: items}
	if len(items) <= limit {
		return out, nil
	}
	out.Items = items[:limit]
	cursor, err := encodeCursor(in.CaseID, out.Items[len(out.Items)-1])
	if err != nil {
		return Output{}, err
	}
	out.NextCursor = &cursor
	return out, nil
}
