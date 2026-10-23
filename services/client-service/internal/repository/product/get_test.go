package product

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestGetMapsProduct(t *testing.T) {
	id := uuid.New()
	db := dbMock{queryRow: func(_ context.Context, query string, args ...any) pgx.Row {
		if !strings.Contains(query, "FROM products") || !strings.Contains(query, "WHERE id = $1") || len(args) != 1 || args[0] != id {
			t.Fatalf("unexpected query: %s %v", query, args)
		}
		return rowMock{scan: func(dest ...any) error {
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
		}}
	}}

	item, err := New(db).Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != id || item.Version != 2 || item.Code != "EXPRESS" || item.IndicativeRateBps == nil || *item.IndicativeRateBps != 1900 {
		t.Fatalf("item = %+v", item)
	}
}

func TestGetMapsNotFound(t *testing.T) {
	db := dbMock{queryRow: func(context.Context, string, ...any) pgx.Row {
		return rowMock{scan: func(...any) error { return pgx.ErrNoRows }}
	}}
	_, err := New(db).Get(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("err = %v", err)
	}
}
