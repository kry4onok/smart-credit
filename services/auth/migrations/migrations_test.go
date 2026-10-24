package migrations_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Syntactic PHC fixture for schema constraints; not a usable demo credential.
const testPasswordHash = "$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func readSQL(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func migration(t *testing.T) (string, string) {
	t.Helper()
	up, down, ok := strings.Cut(readSQL(t, "00001_auth_init.sql"), "-- +goose Down")
	if !ok {
		t.Fatal("migration must contain Up and Down sections")
	}
	return up, down
}

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid AUTH_TEST_DATABASE_URL")
	}
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	if _, err := pool.Exec(t.Context(), "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	up, _ := migration(t)
	if _, err := pool.Exec(t.Context(), up); err != nil {
		t.Fatal(err)
	}
	return pool
}

func fixture(t *testing.T, db *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	orgID, userID := uuid.New(), uuid.New()
	if _, err := db.Exec(t.Context(), `INSERT INTO organizations (id, inn, name) VALUES ($1, '7707083893', 'Тест')`, orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), `INSERT INTO users (id, email, full_name, password_hash, role, org_id)
	    VALUES ($1, 'fixture@example.test', 'Тест', $2, 'CLIENT', $3)`, userID, testPasswordHash, orgID); err != nil {
		t.Fatal(err)
	}
	return orgID, userID
}

func TestMigrationRoundTripIntegration(t *testing.T) {
	db := database(t)
	assertEmpty := func() {
		t.Helper()
		var count int
		if err := db.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM organizations) +
		    (SELECT count(*) FROM users) + (SELECT count(*) FROM refresh_tokens)`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("migration should create empty tables: rows=%d err=%v", count, err)
		}
	}
	assertEmpty()
	_, userID := fixture(t, db)
	if _, err := db.Exec(t.Context(), `INSERT INTO refresh_tokens (token_hash, user_id, expires_at)
	    VALUES ($1, $2, now() + interval '7 days')`, make([]byte, 32), userID); err != nil {
		t.Fatal(err)
	}
	_, down := migration(t)
	if _, err := db.Exec(t.Context(), down); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRow(t.Context(), `SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema()`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("rollback left %d tables, err=%v", remaining, err)
	}
	up, _ := migration(t)
	if _, err := db.Exec(t.Context(), up); err != nil {
		t.Fatal(err)
	}
	assertEmpty()
}

func TestAuthSchemaConstraintsIntegration(t *testing.T) {
	db := database(t)
	orgID, userID := fixture(t, db)
	userInsert := `INSERT INTO users (id, email, full_name, password_hash, role, org_id, can_submit)
	    SELECT $1, $2, 'Тест', password_hash, $3, $4, $5 FROM users WHERE id = $6`
	for _, tc := range []struct {
		name, role string
		org        any
		canSubmit  bool
		code       string
	}{
		{"client without organization", "CLIENT", nil, true, "23514"},
		{"analyst with organization", "ANALYST", orgID, false, "23514"},
		{"admin with submit permission", "ADMIN", nil, true, "23514"},
		{"unknown role", "UNKNOWN", nil, false, "23514"},
		{"foreign organization", "CLIENT", uuid.NewString(), true, "23503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.Exec(t.Context(), userInsert, uuid.New(), "test@example.test", tc.role, tc.org, tc.canSubmit, userID)
			assertSQLState(t, err, tc.code)
		})
	}
	_, err := db.Exec(t.Context(), `INSERT INTO users (id, email, full_name, password_hash, role, org_id)
	    SELECT $1, upper(email), full_name, password_hash, role, org_id FROM users WHERE id = $2`, uuid.New(), userID)
	assertSQLState(t, err, "23505")
	_, err = db.Exec(t.Context(), `UPDATE users SET password_hash = 'plaintext' WHERE id = $1`, userID)
	assertSQLState(t, err, "23514")
	_, err = db.Exec(t.Context(), `INSERT INTO organizations (id, inn, name) VALUES ($1, '7707083893', 'Дубликат')`, uuid.New())
	assertSQLState(t, err, "23505")

	refreshInsert := `INSERT INTO refresh_tokens (token_hash, user_id, created_at, expires_at, revoked_at)
	    VALUES ($1, $2, now(), now() + $3::interval, CASE WHEN $4 THEN now() - interval '1 second' ELSE NULL END)`
	for _, tc := range []struct {
		name, user, lifetime string
		hash                 []byte
		earlyRevocation      bool
		code                 string
	}{
		{"short hash", userID.String(), "7 days", []byte("raw token"), false, "23514"},
		{"too long lifetime", userID.String(), "7 days 1 second", make([]byte, 32), false, "23514"},
		{"zero lifetime", userID.String(), "0 seconds", make([]byte, 32), false, "23514"},
		{"revoked before creation", userID.String(), "7 days", make([]byte, 32), true, "23514"},
		{"missing user", uuid.NewString(), "7 days", make([]byte, 32), false, "23503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.Exec(t.Context(), refreshInsert, tc.hash, tc.user, tc.lifetime, tc.earlyRevocation)
			assertSQLState(t, err, tc.code)
		})
	}
	if _, err := db.Exec(t.Context(), refreshInsert, make([]byte, 32), userID, "7 days", false); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(t.Context(), refreshInsert, make([]byte, 32), userID, "7 days", false)
	assertSQLState(t, err, "23505")
}

func assertSQLState(t *testing.T, err error, want string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != want {
		t.Fatalf("err=%v, want SQLSTATE %s", err, want)
	}
}
