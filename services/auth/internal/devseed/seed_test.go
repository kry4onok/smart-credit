package devseed

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/password"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	selectSeedUsersQuery = `SELECT id, email, role, org_id, can_submit, password_hash FROM users ORDER BY email`
	countSeedUsersQuery  = `SELECT count(*) FROM users`
	insertSessionQuery   = `INSERT INTO refresh_tokens (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`
	countSessionsQuery   = `SELECT count(*) FROM refresh_tokens WHERE user_id = $1`
	createSchemaQuery    = `CREATE SCHEMA %s`
	dropSchemaQuery      = `DROP SCHEMA IF EXISTS %s CASCADE`
)

func TestCredentialsRequirePasswords(t *testing.T) {
	for _, credentials := range []Credentials{
		{},
		{AnalystPassword: "   ", AdminPassword: "admin", ClientPassword: "client"},
		{AnalystPassword: "analyst", ClientPassword: "client"},
		{AnalystPassword: "analyst", AdminPassword: "admin"},
	} {
		if credentials.Validate() == nil {
			t.Fatalf("accepted missing password: %+v", credentials)
		}
	}
	if err := (Credentials{AnalystPassword: "analyst", AdminPassword: "admin", ClientPassword: "client"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRunIntegration(t *testing.T) {
	db := isolatedDatabase(t)
	credentials := Credentials{AnalystPassword: uuid.NewString(), AdminPassword: uuid.NewString(), ClientPassword: uuid.NewString()}
	accounts, err := Run(t.Context(), db, credentials)
	if err != nil || len(accounts) != 3 {
		t.Fatalf("seed accounts = %+v, err = %v", accounts, err)
	}
	firstHashes := readUsers(t, db, credentials)
	if _, err := Run(t.Context(), db, credentials); err != nil {
		t.Fatalf("repeat seed: %v", err)
	}
	secondHashes := readUsers(t, db, credentials)
	for id, hash := range firstHashes {
		if secondHashes[id] != hash {
			t.Fatalf("repeated seed changed password hash for %s", id)
		}
	}
	changed := credentials
	changed.AdminPassword = uuid.NewString()
	if _, err := Run(t.Context(), db, changed); err == nil {
		t.Fatal("seed accepted different password for an existing account")
	}
	finalHashes := readUsers(t, db, credentials)
	for id, hash := range firstHashes {
		if finalHashes[id] != hash {
			t.Fatalf("failed seed changed password hash for %s", id)
		}
	}
	adminUUID := uuid.MustParse(adminID)
	if _, err := db.Exec(t.Context(), insertSessionQuery, make([]byte, 32), adminUUID, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	changed.ResetPasswords = true
	if _, err := Run(t.Context(), db, changed); err != nil {
		t.Fatalf("explicit password reset: %v", err)
	}
	resetHashes := readUsers(t, db, changed)
	for id, hash := range firstHashes {
		if id == adminUUID && resetHashes[id] == hash {
			t.Fatal("admin password was not reset")
		}
		if id != adminUUID && resetHashes[id] != hash {
			t.Fatalf("unchanged account %s was rehashed", id)
		}
	}
	var sessions int
	if err := db.QueryRow(t.Context(), countSessionsQuery, adminUUID).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("admin refresh sessions after reset = %d, err=%v", sessions, err)
	}
}

func readUsers(t *testing.T, db *pgxpool.Pool, credentials Credentials) map[uuid.UUID]string {
	t.Helper()
	rows, err := db.Query(t.Context(), selectSeedUsersQuery)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	secrets := map[string]string{
		"analyst@smart-credit.test": credentials.AnalystPassword,
		"admin@smart-credit.test":   credentials.AdminPassword,
		"client@smart-credit.test":  credentials.ClientPassword,
	}
	result := make(map[uuid.UUID]string)
	for rows.Next() {
		var id uuid.UUID
		var email, role, hash string
		var orgID *uuid.UUID
		var canSubmit bool
		if err := rows.Scan(&id, &email, &role, &orgID, &canSubmit, &hash); err != nil {
			t.Fatal(err)
		}
		secret, ok := secrets[email]
		if !ok || hash == secret {
			t.Fatalf("unexpected or plaintext account %q", email)
		}
		valid, err := password.New().Verify(hash, secret)
		if err != nil || !valid {
			t.Fatalf("account %q has invalid Argon2id hash: %v", email, err)
		}
		switch email {
		case "analyst@smart-credit.test":
			if id.String() != analystID || role != "ANALYST" || orgID != nil || canSubmit {
				t.Fatalf("invalid analyst: %s %s %v %v", id, role, orgID, canSubmit)
			}
		case "admin@smart-credit.test":
			if id.String() != adminID || role != "ADMIN" || orgID != nil || canSubmit {
				t.Fatalf("invalid admin: %s %s %v %v", id, role, orgID, canSubmit)
			}
		case "client@smart-credit.test":
			if id.String() != clientID || role != "CLIENT" || orgID == nil || orgID.String() != organizationID || !canSubmit {
				t.Fatalf("invalid client: %s %s %v %v", id, role, orgID, canSubmit)
			}
		}
		result[id] = hash
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatalf("seeded %d accounts, want 3", len(result))
	}
	return result
}

func isolatedDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("AUTH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set AUTH_TEST_DATABASE_URL to run PostgreSQL seed test")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid AUTH_TEST_DATABASE_URL")
	}
	schema := "auth_seed_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.Exec(ctx, fmt.Sprintf(dropSchemaQuery, pgx.Identifier{schema}.Sanitize())); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(t.Context(), fmt.Sprintf(createSchemaQuery, pgx.Identifier{schema}.Sanitize())); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile("../../migrations/00001_auth_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatal("auth migration has no Down marker")
	}
	if _, err := db.Exec(t.Context(), up); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(t.Context(), countSeedUsersQuery).Scan(&count); err != nil || count != 0 {
		t.Fatalf("isolated auth schema not empty: count=%d err=%v", count, err)
	}
	return db
}
