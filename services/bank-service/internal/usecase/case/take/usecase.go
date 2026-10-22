package take

import (
	"context"
	"errors"
	"fmt"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	"github.com/google/uuid"
)

var (
	ErrNotFound            = errors.New("case not found")
	ErrConflict            = errors.New("case cannot be taken at this version or state")
	ErrIdempotencyConflict = errors.New("idempotency key belongs to a different request")
)

type Input struct {
	CaseID          uuid.UUID
	AnalystID       uuid.UUID
	ExpectedVersion int64
	IdempotencyKey  uuid.UUID
}

type Repository interface {
	Take(context.Context, Input) (model.Case, error)
}

type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}

type UseCase struct {
	repository   Repository
	transactions TransactionManager
}

func New(repository Repository, transactions TransactionManager) *UseCase {
	return &UseCase{repository: repository, transactions: transactions}
}

func (uc *UseCase) Execute(ctx context.Context, input Input) (model.Case, error) {
	var result model.Case
	err := uc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		result, err = uc.repository.Take(txCtx, input)
		return err
	})
	if err != nil {
		return model.Case{}, fmt.Errorf("take case: %w", err)
	}
	return result, nil
}
