package list

import (
	"context"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	ListCases(ctx context.Context, query RepositoryQuery) ([]model.CaseListItem, error)
}

type Input struct {
	Limit         *int
	Cursor        *string
	Status        *model.CaseStatus
	ProductID     *uuid.UUID
	Assignee      *string
	CurrentUserID uuid.UUID
}

type Output struct {
	Items      []model.CaseListItem
	NextCursor *string
}

type RepositoryQuery struct {
	Limit     int
	After     *Cursor
	Status    *model.CaseStatus
	ProductID *uuid.UUID
	Assignee  AssigneeFilter
}

type Cursor struct {
	ManualReview bool
	CreatedAt    time.Time
	ID           uuid.UUID
}

type AssigneeFilter struct {
	Kind AssigneeFilterKind
	ID   uuid.UUID
}

type AssigneeFilterKind uint8

const (
	AssigneeAny AssigneeFilterKind = iota
	AssigneeUnassigned
	AssigneeUser
)
