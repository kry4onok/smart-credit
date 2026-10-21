package caserepository

import listcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/list"

type Repository struct {
	db Queryer
}

func New(db Queryer) *Repository {
	return &Repository{db: db}
}

var _ listcase.Repository = (*Repository)(nil)
