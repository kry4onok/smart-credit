package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	setValidEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTP.Address != ":8080" ||
		cfg.HTTP.ReadHeaderTimeout != 5*time.Second ||
		cfg.HTTP.ReadTimeout != 30*time.Second ||
		cfg.HTTP.WriteTimeout != 30*time.Second ||
		cfg.HTTP.IdleTimeout != 60*time.Second ||
		cfg.HTTP.MaxHeaderBytes != 1<<20 {
		t.Errorf("HTTP config = %+v", cfg.HTTP)
	}
	if cfg.Database.ConnectTimeout != 5*time.Second ||
		cfg.Database.MaxConnections != 10 ||
		cfg.Database.MinConnections != 1 ||
		cfg.Database.MaxConnLifetime != 30*time.Minute ||
		cfg.Database.MaxConnIdleTime != 5*time.Minute ||
		cfg.Database.HealthCheckPeriod != time.Minute {
		t.Errorf("Database config = %+v", cfg.Database)
	}
	if cfg.ReadinessTimeout != time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf(
			"timeouts: readiness=%s shutdown=%s",
			cfg.ReadinessTimeout,
			cfg.ShutdownTimeout,
		)
	}
}

func TestLoadDemoAnalystID(t *testing.T) {
	setValidEnvironment(t)
	id := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	t.Setenv("BANK_DEMO_ANALYST_ID", id)
	cfg, err := Load()
	if err != nil || cfg.DemoAnalystID != id {
		t.Fatalf("DemoAnalystID=%q, err=%v", cfg.DemoAnalystID, err)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "missing database URL",
			env:  map[string]string{"BANK_DATABASE_URL": ""},
			want: "BANK_DATABASE_URL",
		},
		{
			name: "unsupported database scheme",
			env:  map[string]string{"BANK_DATABASE_URL": "mysql://bank:bank@localhost/bank_db"},
			want: "BANK_DATABASE_URL must be a postgres URL",
		},
		{
			name: "invalid HTTP address",
			env:  map[string]string{"BANK_HTTP_ADDR": "8080"},
			want: "BANK_HTTP_ADDR must have host:port form",
		},
		{
			name: "minimum connections exceed maximum",
			env: map[string]string{
				"BANK_DB_MAX_CONNS": "2",
				"BANK_DB_MIN_CONNS": "3",
			},
			want: "MinConnections",
		},
		{
			name: "readiness exceeds write timeout",
			env:  map[string]string{"BANK_READINESS_TIMEOUT": "30s"},
			want: "ReadinessTimeout",
		},
		{
			name: "invalid demo analyst ID",
			env:  map[string]string{"BANK_DEMO_ANALYST_ID": "invalid"},
			want: "BANK_DEMO_ANALYST_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnvironment(t)
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func setValidEnvironment(t *testing.T) {
	t.Helper()

	values := map[string]string{
		"BANK_HTTP_ADDR":                ":8080",
		"BANK_HTTP_READ_HEADER_TIMEOUT": "5s",
		"BANK_HTTP_READ_TIMEOUT":        "30s",
		"BANK_HTTP_WRITE_TIMEOUT":       "30s",
		"BANK_HTTP_IDLE_TIMEOUT":        "60s",
		"BANK_HTTP_MAX_HEADER_BYTES":    "1048576",
		"BANK_DATABASE_URL":             "postgres://bank:bank@localhost/bank_db",
		"BANK_DB_CONNECT_TIMEOUT":       "5s",
		"BANK_DB_MAX_CONNS":             "10",
		"BANK_DB_MIN_CONNS":             "1",
		"BANK_DB_MAX_CONN_LIFETIME":     "30m",
		"BANK_DB_MAX_CONN_IDLE_TIME":    "5m",
		"BANK_DB_HEALTH_CHECK_PERIOD":   "1m",
		"BANK_READINESS_TIMEOUT":        "1s",
		"BANK_SHUTDOWN_TIMEOUT":         "10s",
		"BANK_DEMO_ANALYST_ID":          "",
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
}
