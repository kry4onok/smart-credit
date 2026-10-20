package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("BANK_DATABASE_URL", "postgres://bank_user:bank_local@127.0.0.1:5432/bank_db?sslmode=disable")
	t.Setenv("BANK_HTTP_ADDR", "")
	t.Setenv("BANK_DB_CONNECT_TIMEOUT", "")
	t.Setenv("BANK_DB_MAX_CONNS", "")
	t.Setenv("BANK_DB_MIN_CONNS", "")
	t.Setenv("BANK_SHUTDOWN_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddress != ":8080" {
		t.Errorf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":8080")
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %s, want %s", cfg.ShutdownTimeout, 10*time.Second)
	}
	if cfg.Database.ConnectTimeout != 5*time.Second {
		t.Errorf("Database.ConnectTimeout = %s, want %s", cfg.Database.ConnectTimeout, 5*time.Second)
	}
	if cfg.Database.MaxConnections != 10 || cfg.Database.MinConnections != 1 {
		t.Errorf("database pool size = %d..%d, want 1..10", cfg.Database.MinConnections, cfg.Database.MaxConnections)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		dbURL   string
		address string
		connect string
		max     string
		min     string
		timeout string
		want    string
	}{
		{
			name: "missing database URL",
			want: "BANK_DATABASE_URL is required",
		},
		{
			name:  "unsupported database scheme",
			dbURL: "mysql://bank:bank@localhost/bank_db",
			want:  "scheme must be postgres or postgresql",
		},
		{
			name:    "invalid HTTP address",
			dbURL:   "postgres://bank:bank@localhost/bank_db",
			address: "8080",
			want:    "must have host:port form",
		},
		{
			name:    "invalid database connect timeout",
			dbURL:   "postgres://bank:bank@localhost/bank_db",
			connect: "0s",
			want:    "BANK_DB_CONNECT_TIMEOUT must be a positive duration",
		},
		{
			name:  "minimum connections exceed maximum",
			dbURL: "postgres://bank:bank@localhost/bank_db",
			max:   "2",
			min:   "3",
			want:  "BANK_DB_MIN_CONNS must not exceed BANK_DB_MAX_CONNS",
		},
		{
			name:    "non-positive shutdown timeout",
			dbURL:   "postgres://bank:bank@localhost/bank_db",
			timeout: "0s",
			want:    "must be a positive duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BANK_DATABASE_URL", tt.dbURL)
			t.Setenv("BANK_HTTP_ADDR", tt.address)
			t.Setenv("BANK_DB_CONNECT_TIMEOUT", tt.connect)
			t.Setenv("BANK_DB_MAX_CONNS", tt.max)
			t.Setenv("BANK_DB_MIN_CONNS", tt.min)
			t.Setenv("BANK_SHUTDOWN_TIMEOUT", tt.timeout)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
