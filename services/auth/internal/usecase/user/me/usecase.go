package me

import (
	"context"
	"errors"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

type Usecase struct{ repository Repository }

func New(repository Repository) *Usecase { return &Usecase{repository: repository} }

func (u *Usecase) Execute(ctx context.Context, userID uuid.UUID) (model.CurrentUser, error) {
	if userID == uuid.Nil {
		return model.CurrentUser{}, domain.ErrUnauthorized
	}
	user, err := u.repository.FetchCurrent(ctx, userID)
	if errors.Is(err, domain.ErrUserNotFound) {
		return model.CurrentUser{}, domain.ErrUnauthorized
	}
	if err != nil {
		return model.CurrentUser{}, err
	}
	if !user.Active {
		return model.CurrentUser{}, domain.ErrUnauthorized
	}
	return user, nil
}
