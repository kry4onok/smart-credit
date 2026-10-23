package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Tx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

type DB interface {
	Begin(context.Context) (Tx, error)
}

type Pool interface {
	Begin(context.Context) (pgx.Tx, error)
}

type poolAdapter struct{ pool Pool }

func Adapt(pool Pool) DB { return poolAdapter{pool: pool} }

func (a poolAdapter) Begin(ctx context.Context) (Tx, error) { return a.pool.Begin(ctx) }
