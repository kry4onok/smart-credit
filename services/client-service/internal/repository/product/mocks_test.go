package product

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type rowMock struct{ scan func(...any) error }

func (r rowMock) Scan(dest ...any) error { return r.scan(dest...) }

type rowsMock struct {
	scans []func(...any) error
	index int
	err   error
}

func (r *rowsMock) Close() {}

func (r *rowsMock) Err() error { return r.err }

func (r *rowsMock) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *rowsMock) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *rowsMock) Next() bool {
	if r.index >= len(r.scans) {
		return false
	}
	r.index++
	return true
}

func (r *rowsMock) Scan(dest ...any) error { return r.scans[r.index-1](dest...) }

func (r *rowsMock) Values() ([]any, error) { return nil, nil }

func (r *rowsMock) RawValues() [][]byte { return nil }

func (r *rowsMock) Conn() *pgx.Conn { return nil }

func (r *rowsMock) TypeMap() *pgtype.Map { return nil }
