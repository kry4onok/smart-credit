package product

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type dbMock struct {
	queryRow func(context.Context, string, ...any) pgx.Row
	query    func(context.Context, string, ...any) (pgx.Rows, error)
}

func (m dbMock) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return m.queryRow(ctx, query, args...)
}

func (m dbMock) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return m.query(ctx, query, args...)
}

func TestListRanksPurposeAndMapsProduct(t *testing.T) {
	purpose := "WORKING_CAPITAL"
	id := uuid.New()
	db := dbMock{
		queryRow: func(_ context.Context, query string, args ...any) pgx.Row {
			if !strings.Contains(query, "aggregate_id = 'catalog'") || len(args) != 0 {
				t.Fatalf("unexpected version query: %s %v", query, args)
			}
			return rowMock{scan: func(dest ...any) error {
				*dest[0].(*int64) = 4
				return nil
			}}
		},
		query: func(_ context.Context, query string, args ...any) (pgx.Rows, error) {
			if !strings.Contains(query, "ANY(purpose_tags)") || !strings.Contains(query, "ORDER BY CASE") || len(args) != 1 || args[0] != purpose {
				t.Fatalf("unexpected product query: %s %v", query, args)
			}
			return &rowsMock{scans: []func(...any) error{func(dest ...any) error {
				*dest[0].(*uuid.UUID) = id
				*dest[1].(*int64) = 2
				*dest[2].(*string) = "EXPRESS"
				*dest[3].(*string) = "Экспресс"
				*dest[4].(*string) = "Оборотный кредит"
				*dest[5].(*[]string) = []string{"WORKING_CAPITAL"}
				*dest[6].(*int64) = 100
				*dest[7].(*int64) = 200
				*dest[8].(*int) = 3
				*dest[9].(*int) = 12
				rate := 1900
				*dest[10].(**int) = &rate
				*dest[11].(*[]string) = []string{"BANK_STATEMENT"}
				return nil
			}}}, nil
		},
	}

	catalog, err := New(db).List(context.Background(), &purpose)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Version != 4 || len(catalog.Items) != 1 || catalog.Items[0].ID != id || catalog.Items[0].IndicativeRateBps == nil || *catalog.Items[0].IndicativeRateBps != 1900 {
		t.Fatalf("catalog = %+v", catalog)
	}
}

func TestListMapsDatabaseFailure(t *testing.T) {
	db := dbMock{queryRow: func(context.Context, string, ...any) pgx.Row {
		return rowMock{scan: func(...any) error { return errors.New("database unavailable") }}
	}}
	_, err := New(db).List(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "fetch catalog version") {
		t.Fatalf("err = %v", err)
	}
}
