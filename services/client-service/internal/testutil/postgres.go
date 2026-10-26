// Package testutil provides isolated PostgreSQL schemas for integration tests.
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres skips the test unless CLIENT_TEST_DATABASE_URL is set. Each test
// uses a new schema, so tests never replace existing application tables.
func Postgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CLIENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CLIENT_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "client_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	applyMigration(t, pool, "00001_client_init.sql")
	applyMigration(t, pool, "00002_profile_requirements.sql")
	return pool
}

func applyMigration(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate client migrations")
	}
	contents, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(contents), "-- +goose Down")
	if _, err := pool.Exec(t.Context(), up); err != nil {
		t.Fatal(err)
	}
}
