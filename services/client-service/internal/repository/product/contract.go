package product

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type DB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
