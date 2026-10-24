package config

import (
	"strings"
	"testing"
	"time"
)

func setEnvironment(t *testing.T) {
	t.Helper()
	for name, value := range map[string]string{
		"AUTH_DATABASE_URL": "postgres://auth_user:auth_local@127.0.0.1:5432/auth_db?sslmode=disable",
		"AUTH_HTTP_ADDR":    ":8082", "AUTH_HTTP_READ_HEADER_TIMEOUT": "5s", "AUTH_HTTP_READ_TIMEOUT": "30s",
		"AUTH_HTTP_WRITE_TIMEOUT": "30s", "AUTH_HTTP_IDLE_TIMEOUT": "60s", "AUTH_HTTP_MAX_HEADER_BYTES": "1048576",
		"AUTH_DB_CONNECT_TIMEOUT": "5s", "AUTH_DB_MAX_CONNS": "10", "AUTH_DB_MIN_CONNS": "1",
		"AUTH_DB_MAX_CONN_LIFETIME": "30m", "AUTH_DB_MAX_CONN_IDLE_TIME": "5m", "AUTH_DB_HEALTH_CHECK_PERIOD": "1m",
		"AUTH_READINESS_TIMEOUT": "1s", "AUTH_SHUTDOWN_TIMEOUT": "10s", "AUTH_JWT_ISSUER": "smart-credit-auth",
		"AUTH_JWT_AUDIENCE": "smart-credit", "AUTH_JWT_ACCESS_TTL": "15m", "AUTH_JWT_KEY_ID": "auth-v1",
		"AUTH_JWT_PRIVATE_KEY_PATH": ".data/auth/private.pem", "AUTH_REFRESH_COOKIE_NAME": "smart_credit_refresh",
		"AUTH_REFRESH_COOKIE_PATH": "/api/auth", "AUTH_REFRESH_COOKIE_SECURE": "false",
	} {
		t.Setenv(name, value)
	}
}

func TestLoad(t *testing.T) {
	setEnvironment(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Address != ":8082" || cfg.HTTP.ReadHeaderTimeout != 5*time.Second || cfg.HTTP.WriteTimeout != 30*time.Second || cfg.HTTP.MaxHeaderBytes != 1<<20 ||
		cfg.Database.MaxConnections != 10 || cfg.Database.MinConnections != 1 || cfg.ReadinessTimeout != time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("config=%+v", cfg)
	}
	if cfg.JWT.Issuer != "smart-credit-auth" || cfg.JWT.Audience != "smart-credit" || cfg.JWT.AccessTTL != 15*time.Minute || cfg.Cookie.Secure || cfg.Cookie.Path != "/api/auth" {
		t.Fatalf("JWT=%+v cookie=%+v", cfg.JWT, cfg.Cookie)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name, value, want string
	}{
		{"AUTH_DATABASE_URL", "", "AUTH_DATABASE_URL"},
		{"AUTH_DATABASE_URL", "mysql://auth:private-secret@localhost/auth_db", "AUTH_DATABASE_URL"},
		{"AUTH_DATABASE_URL", "postgres://auth:private-secret@localhost", "AUTH_DATABASE_URL"},
		{"AUTH_DATABASE_URL", "postgres://auth:private-secret%zz@localhost/auth_db", "AUTH_DATABASE_URL"},
		{"AUTH_HTTP_ADDR", "8082", "AUTH_HTTP_ADDR"},
		{"AUTH_HTTP_ADDR", ":0", "AUTH_HTTP_ADDR"},
		{"AUTH_HTTP_ADDR", ":65536", "AUTH_HTTP_ADDR"},
		{"AUTH_HTTP_READ_TIMEOUT", "0s", "ReadTimeout"},
		{"AUTH_READINESS_TIMEOUT", "30s", "ReadinessTimeout"},
		{"AUTH_SHUTDOWN_TIMEOUT", "-1s", "ShutdownTimeout"},
		{"AUTH_DB_MAX_CONNS", "0", "MaxConnections"},
		{"AUTH_DB_MIN_CONNS", "11", "MinConnections"},
		{"AUTH_JWT_ACCESS_TTL", "1ms", "AUTH_JWT_ACCESS_TTL"},
		{"AUTH_JWT_KEY_ID", "  ", "AUTH_JWT_KEY_ID"},
		{"AUTH_JWT_PRIVATE_KEY_PATH", "  ", "AUTH_JWT_PRIVATE_KEY_PATH"},
		{"AUTH_REFRESH_COOKIE_NAME", "bad name", "cookie"},
		{"AUTH_REFRESH_COOKIE_PATH", "api/auth", "AUTH_REFRESH_COOKIE_PATH"},
		{"AUTH_REFRESH_COOKIE_PATH", "/api/auth;bad", "cookie"},
		{"AUTH_REFRESH_COOKIE_NAME", "__Secure-refresh", "AUTH_REFRESH_COOKIE_SECURE"},
		{"AUTH_REFRESH_COOKIE_NAME", "__Host-refresh", "__Host-"},
	} {
		t.Run(tc.name+"/"+tc.want+"/"+tc.value, func(t *testing.T) {
			setEnvironment(t)
			t.Setenv(tc.name, tc.value)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "private-secret") {
				t.Fatalf("invalid environment was not safely rejected: %v", err)
			}
		})
	}
}

func TestLoadHTTPSCookie(t *testing.T) {
	setEnvironment(t)
	t.Setenv("AUTH_REFRESH_COOKIE_NAME", "__Host-refresh")
	t.Setenv("AUTH_REFRESH_COOKIE_SECURE", "true")
	t.Setenv("AUTH_REFRESH_COOKIE_PATH", "/")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}
