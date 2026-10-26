package info

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Repository interface {
	ProvideInfo(context.Context, model.ProvideInfoParams) (model.ActionAccepted, error)
}

type UseCase struct{ repository Repository }

func New(repository Repository) *UseCase { return &UseCase{repository: repository} }

func (u *UseCase) Execute(ctx context.Context, params model.ProvideInfoParams) (model.ActionAccepted, error) {
	if params.OrgID == uuid.Nil || params.UserID == uuid.Nil ||
		params.ApplicationID == uuid.Nil || params.RequestID == uuid.Nil ||
		params.IdempotencyID == uuid.Nil || len(params.RequestHash) != 32 {
		return model.ActionAccepted{}, domain.ErrInvalidInfoResponse
	}
	if params.Company != nil {
		if err := domain.ValidateCompany(*params.Company); err != nil {
			if params.Company.ProfileSchemaVersion != nil {
				return model.ActionAccepted{}, err
			}
			return model.ActionAccepted{}, domain.ErrInvalidInfoResponse
		}
	}
	if params.Fields != nil {
		if err := domain.ValidateApplicationPatch(*params.Fields); err != nil {
			return model.ActionAccepted{}, domain.ErrInvalidInfoResponse
		}
	}
	if params.Company == nil && params.Fields == nil && !params.DocumentsSpecified {
		return model.ActionAccepted{}, domain.ErrInvalidInfoResponse
	}
	return u.repository.ProvideInfo(ctx, params)
}
