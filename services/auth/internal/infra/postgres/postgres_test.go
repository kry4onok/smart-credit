package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
)

func TestMalformedURLDoesNotExposeCredentials(t *testing.T) {
	for _, dsn := range []string{
		"postgres://auth:private-secret%zz@localhost/auth_db",
		"postgres://auth:private-secret@localhost/auth_db?port=invalid",
	} {
		pool, err := New(t.Context(), config.DatabaseConfig{URL: dsn})
		if pool != nil {
			pool.Close()
		}
		if err == nil || strings.Contains(err.Error(), "private-secret") || !strings.Contains(err.Error(), "AUTH_DATABASE_URL") {
			t.Fatalf("malformed URL was not safely rejected: %v", err)
		}
	}
}

func TestPoolCanStartWithoutAvailableDatabase(t *testing.T) {
	pool, err := New(t.Context(), config.DatabaseConfig{
		URL: "postgres://auth:auth@127.0.0.1:1/auth_db?sslmode=disable", ConnectTimeout: 10 * time.Millisecond,
		MaxConnections: 1, MaxConnLifetime: time.Minute, MaxConnIdleTime: time.Minute, HealthCheckPeriod: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
}
