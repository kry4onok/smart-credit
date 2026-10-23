package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type DB interface {
	Begin(context.Context) (Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Pool interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type poolAdapter struct{ pool Pool }

func Adapt(pool Pool) DB { return poolAdapter{pool: pool} }

func (a poolAdapter) Begin(ctx context.Context) (Tx, error) { return a.pool.Begin(ctx) }

func (a poolAdapter) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return a.pool.Exec(ctx, query, args...)
}

func (a poolAdapter) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return a.pool.Query(ctx, query, args...)
}

func (a poolAdapter) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return a.pool.QueryRow(ctx, query, args...)
}
