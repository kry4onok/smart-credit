package container

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/infra/jwt"
)

func testConfig(path string) config.Config {
	return config.Config{
		JWT: config.JWTConfig{Issuer: "smart-credit-auth", Audience: "smart-credit", AccessTTL: 15 * time.Minute,
			KeyID: "auth-v1", PrivateKeyPath: path},
		Database: config.DatabaseConfig{URL: "postgres://auth:auth@127.0.0.1:1/auth_db?sslmode=disable",
			ConnectTimeout: 10 * time.Millisecond, MaxConnections: 1,
			MaxConnLifetime: time.Minute, MaxConnIdleTime: time.Minute, HealthCheckPeriod: time.Minute},
		ReadinessTimeout: 10 * time.Millisecond,
	}
}

func TestJWKSWithoutDatabaseOrAuthorization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.pem")
	if err := jwt.GenerateKey(path); err != nil {
		t.Fatal(err)
	}
	c, err := New(t.Context(), testConfig(path))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	w := httptest.NewRecorder()
	c.HTTPHandler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" ||
		w.Header().Get("Cache-Control") != "public, max-age=300" || w.Header().Get("X-Trace-ID") == "" {
		t.Fatalf("status=%d headers=%v", w.Code, w.Header())
	}
	var body struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Keys) != 1 || len(body.Keys[0]) != 6 {
		t.Fatal("JWKS must contain one public key and exactly the contract's six parameters")
	}
	key := body.Keys[0]
	public := c.Tokens.PublicKey()
	if key["alg"] != "RS256" || key["kty"] != "RSA" || key["use"] != "sig" ||
		key["kid"] != public.KeyID || key["n"] != public.N || key["e"] != public.E {
		t.Fatal("HTTP JWKS does not match signing key")
	}
	w = httptest.NewRecorder()
	c.HTTPHandler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal("test database unexpectedly ready")
	}
}

func TestStartupRequiresValidKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.pem")
	if c, err := New(t.Context(), testConfig(path)); err == nil || c != nil {
		if c != nil {
			c.Close()
		}
		t.Fatal("startup succeeded without a signing key")
	}
}
