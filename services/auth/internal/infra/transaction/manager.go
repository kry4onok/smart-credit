package transaction

import (
	"context"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Executor interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}
type Manager struct{ db Beginner }
type contextKey struct{}

func New(db Beginner) *Manager { return &Manager{db: db} }

func (m *Manager) Within(ctx context.Context, action func(context.Context) error) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return postgres.OperationError("begin auth transaction", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err := action(context.WithValue(ctx, contextKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return postgres.OperationError("commit auth transaction", err)
	}
	return nil
}

func ExecutorFor(ctx context.Context, fallback Executor) Executor {
	if tx, ok := ctx.Value(contextKey{}).(pgx.Tx); ok {
		return tx
	}
	return fallback
}
