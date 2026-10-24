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
	if cfg.KafkaBrokers != "127.0.0.1:9092" || cfg.KafkaGroup != "bank-service" {
		t.Errorf("Kafka config = brokers %q, group %q", cfg.KafkaBrokers, cfg.KafkaGroup)
	}
	if cfg.JWT.JWKSURL != "http://127.0.0.1:8082/.well-known/jwks.json" ||
		cfg.JWT.Issuer != "smart-credit-auth" || cfg.JWT.Audience != "smart-credit" ||
		cfg.JWT.Timeout != 2*time.Second || cfg.JWT.CacheTTL != 5*time.Minute {
		t.Errorf("JWT config = %+v", cfg.JWT)
	}
}

func TestLoadJWT(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("BANK_JWKS_URL", "http://auth:8080/.well-known/jwks.json")
	cfg, err := Load()
	if err != nil || cfg.JWT.JWKSURL != "http://auth:8080/.well-known/jwks.json" {
		t.Fatalf("JWT=%+v, err=%v", cfg.JWT, err)
	}
}

func TestLoadAssistantTarget(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("BANK_ASSISTANT_GRPC_ADDR", "127.0.0.1:50052")
	cfg, err := Load()
	if err != nil || cfg.AssistantTarget != "127.0.0.1:50052" {
		t.Fatalf("AssistantTarget=%q, err=%v", cfg.AssistantTarget, err)
	}
}

func TestLoadAssistantTimeout(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("BANK_ASSISTANT_TIMEOUT", "20s")
	cfg, err := Load()
	if err != nil || cfg.AssistantTimeout != 20*time.Second {
		t.Fatalf("AssistantTimeout=%s, err=%v", cfg.AssistantTimeout, err)
	}
	t.Setenv("BANK_ASSISTANT_TIMEOUT", "30s")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "BANK_ASSISTANT_TIMEOUT") {
		t.Fatalf("expected timeout validation error, got %v", err)
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
			name: "invalid JWKS URL",
			env:  map[string]string{"BANK_JWKS_URL": "invalid"},
			want: "JWKSURL",
		},
		{
			name: "invalid Kafka brokers",
			env:  map[string]string{"BANK_KAFKA_BROKERS": "localhost"},
			want: "BANK_KAFKA_BROKERS",
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
		"BANK_JWKS_URL":                 "http://127.0.0.1:8082/.well-known/jwks.json",
		"BANK_JWT_ISSUER":               "smart-credit-auth",
		"BANK_JWT_AUDIENCE":             "smart-credit",
		"BANK_JWKS_TIMEOUT":             "2s",
		"BANK_JWKS_CACHE_TTL":           "5m",
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
}
