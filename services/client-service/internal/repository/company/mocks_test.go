package company

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type dbMock struct {
	query func(context.Context, string, ...any) pgx.Row
}

func (m dbMock) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return m.query(ctx, query, args...)
}

type rowMock struct{ scan func(...any) error }

func (m rowMock) Scan(dest ...any) error { return m.scan(dest...) }
