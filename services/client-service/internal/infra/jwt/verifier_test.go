package jwt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/config"
	"github.com/google/uuid"
)

func TestVerifyAndRotateJWKS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var active atomic.Value
	active.Store("first")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
			"kid": active.Load(), "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	defer server.Close()
	verifier := New(config.JWTConfig{
		JWKSURL: server.URL, Issuer: "issuer", Audience: "audience",
		Timeout: time.Second, CacheTTL: time.Minute,
	})
	userID, orgID := uuid.New(), uuid.New()
	payload := map[string]any{"sub": userID.String(), "org_id": orgID.String(), "role": "CLIENT",
		"iss": "issuer", "aud": []string{"other", "audience"}, "exp": time.Now().Add(time.Hour).Unix()}
	token := signToken(t, key, "first", "RS256", payload)
	for range 2 {
		principal, err := verifier.Verify(context.Background(), token)
		if err != nil || principal.OrgID != orgID || principal.UserID != userID {
			t.Fatalf("principal = %+v, err = %v", principal, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("JWKS requests = %d", calls.Load())
	}
	for _, role := range []string{"ANALYST", "ADMIN"} {
		staffClaims := map[string]any{"sub": userID.String(), "role": role,
			"iss": "issuer", "aud": "audience", "exp": time.Now().Add(time.Hour).Unix()}
		principal, err := verifier.Verify(t.Context(), signToken(t, key, "first", "RS256", staffClaims))
		if err != nil || principal.Role != role || principal.UserID != userID || principal.OrgID != uuid.Nil {
			t.Fatalf("staff JWT without org_id: principal=%+v, err=%v", principal, err)
		}
	}
	active.Store("second")
	if _, err := verifier.Verify(context.Background(), signToken(t, key, "second", "RS256", payload)); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("unknown kid did not refresh JWKS")
	}
	verifier.expires = time.Now().Add(-time.Second)
	if _, err := verifier.Verify(context.Background(), signToken(t, key, "second", "RS256", payload)); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatal("expired cache did not refresh JWKS")
	}

	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"expired", "exp", time.Now().Add(-time.Minute).Unix()},
		{"issuer", "iss", "wrong"}, {"audience", "aud", "wrong"},
		{"organization", "org_id", "invalid"}, {"user", "sub", uuid.Nil.String()},
		{"not yet valid", "nbf", time.Now().Add(time.Hour).Unix()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := make(map[string]any)
			for k, v := range payload {
				changed[k] = v
			}
			changed[tc.field] = tc.value
			if _, err := verifier.Verify(context.Background(), signToken(t, key, "second", "RS256", changed)); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	if _, err := verifier.Verify(context.Background(), signToken(t, key, "second", "none", payload)); err == nil {
		t.Fatal("wrong algorithm accepted")
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), signToken(t, other, "second", "RS256", payload)); err == nil {
		t.Fatal("wrong signature accepted")
	}
	server.Close()
	verifier.expires = time.Now().Add(-time.Second)
	if _, err := verifier.Verify(context.Background(), token); err == nil {
		t.Fatal("unavailable JWKS accepted")
	}
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid, algorithm string, payload map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": algorithm, "kid": kid})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	data := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	hash := sha256.Sum256([]byte(data))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	return data + "." + base64.RawURLEncoding.EncodeToString(signature)
}
