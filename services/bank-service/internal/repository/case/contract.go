package caserepository

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type rowScanner interface {
	Scan(dest ...any) error
}
