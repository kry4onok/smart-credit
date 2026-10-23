package validation

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Claim struct {
	CaseID         uuid.UUID
	Version        int64
	ProductID      uuid.UUID
	ProductVersion int64
	Snapshot       Snapshot
}

type Repository interface {
	ClaimNext(context.Context) (Claim, bool, error)
	LoadProduct(context.Context, Claim) (Product, error)
	Complete(context.Context, Claim, Outcome) error
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

// RunOnce holds one row lock while it evaluates local snapshots and saves the
// transition. It makes no network calls inside the transaction.
func (uc *UseCase) RunOnce(ctx context.Context) (bool, error) {
	var found bool
	err := uc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		claim, hasCase, err := uc.repository.ClaimNext(txCtx)
		if err != nil || !hasCase {
			return err
		}
		found = true
		product, err := uc.repository.LoadProduct(txCtx, claim)
		if err != nil {
			return err
		}
		return uc.repository.Complete(txCtx, claim, Evaluate(claim.Snapshot, product))
	})
	if err != nil {
		return found, fmt.Errorf("validate next case: %w", err)
	}
	return found, nil
}
