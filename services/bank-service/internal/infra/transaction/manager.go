package transaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const rollbackTimeout = 5 * time.Second

var ErrNested = errors.New("nested transaction")

type Queryer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Manager struct {
	db Beginner
}

type txContextKey struct{}

func New(db Beginner) *Manager {
	return &Manager{db: db}
}

func (m *Manager) WithinTransaction(ctx context.Context, fn func(context.Context) error) (err error) {
	if _, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return ErrNested
	}

	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}

		rollbackContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()

		rollbackErr := tx.Rollback(rollbackContext)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
	}()

	if err = fn(context.WithValue(ctx, txContextKey{}, tx)); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true

	return nil
}

func QueryerFromContext(ctx context.Context, fallback Queryer) Queryer {
	if tx, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return tx
	}

	return fallback
}
