package update

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (u *Service) Execute(ctx context.Context, orgID uuid.UUID, profile model.Company, expectedVersion *int64) (model.Company, error) {
	if err := domain.ValidateCompany(profile); err != nil {
		return model.Company{}, err
	}
	if expectedVersion == nil {
		return u.repository.Create(ctx, orgID, profile)
	}
	return u.repository.Update(ctx, orgID, profile, *expectedVersion)
}
