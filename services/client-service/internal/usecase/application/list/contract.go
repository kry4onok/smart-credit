package list

import (
	"context"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	List(context.Context, RepositoryQuery) ([]model.ApplicationListItem, error)
}

type Input struct {
	OrgID  uuid.UUID
	Limit  *int
	Cursor *string
	Status *string
}

type Output struct {
	Items      []model.ApplicationListItem
	NextCursor *string
}

type RepositoryQuery struct {
	OrgID  uuid.UUID
	Limit  int
	After  *Cursor
	Status *string
}

type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }
