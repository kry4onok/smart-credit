package list

import (
	"context"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
)

type repositoryMock struct {
	listCases func(context.Context, RepositoryQuery) ([]model.CaseListItem, error)
}

func (m repositoryMock) ListCases(
	ctx context.Context,
	query RepositoryQuery,
) ([]model.CaseListItem, error) {
	return m.listCases(ctx, query)
}
