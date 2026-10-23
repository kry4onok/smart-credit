package company

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type DB interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}
