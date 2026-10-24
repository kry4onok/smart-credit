package jwt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/config"
	"github.com/google/uuid"
)

func TestVerifyAuthTokenAndRotateJWKS(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	var kid atomic.Value
	kid.Store("first")
	var available atomic.Bool
	available.Store(true)
	verifier := New(config.JWTConfig{
		JWKSURL: "http://auth.test/.well-known/jwks.json", Issuer: "smart-credit-auth", Audience: "smart-credit",
		Timeout: time.Second, CacheTTL: time.Minute,
	})
	verifier.client.Transport = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		requests.Add(1)
		if !available.Load() {
			return nil, errors.New("JWKS unavailable")
		}
		body, err := json.Marshal(map[string]any{"keys": []any{map[string]any{
			"kid": kid.Load(), "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})
	userID := uuid.New()
	now := time.Now().Unix()
	claims := map[string]any{"sub": userID.String(), "role": "ANALYST", "iss": "smart-credit-auth",
		"aud": "smart-credit", "iat": now, "exp": now + 3600}
	token := signToken(t, key, "first", claims)
	for range 2 {
		principal, err := verifier.Verify(t.Context(), token)
		if err != nil || principal.UserID != userID || principal.Role != "ANALYST" {
			t.Fatalf("principal = %+v, err = %v", principal, err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("JWKS requests = %d, want 1", requests.Load())
	}
	kid.Store("second")
	if _, err := verifier.Verify(t.Context(), signToken(t, key, "second", claims)); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatal("unknown kid did not refresh JWKS")
	}
	verifier.expires = time.Now().Add(-time.Second)
	if _, err := verifier.Verify(t.Context(), signToken(t, key, "second", claims)); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 3 {
		t.Fatal("expired JWKS cache did not refresh")
	}

	for _, test := range []struct {
		name  string
		field string
		value any
	}{
		{"expired", "exp", now - 1},
		{"future issued", "iat", now + 3600},
		{"missing issued", "iat", 0},
		{"wrong issuer", "iss", "other"},
		{"wrong audience", "aud", "other"},
		{"future not before", "nbf", now + 3600},
		{"invalid subject", "sub", uuid.Nil.String()},
		{"invalid role", "role", "OTHER"},
		{"employee organization", "org_id", uuid.NewString()},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := make(map[string]any, len(claims))
			for k, v := range claims {
				changed[k] = v
			}
			changed[test.field] = test.value
			if _, err := verifier.Verify(t.Context(), signToken(t, key, "second", changed)); err == nil {
				t.Fatal("invalid access token accepted")
			}
		})
	}
	clientClaims := make(map[string]any, len(claims)+1)
	for k, v := range claims {
		clientClaims[k] = v
	}
	clientClaims["role"] = "CLIENT"
	clientClaims["org_id"] = uuid.NewString()
	principal, err := verifier.Verify(t.Context(), signToken(t, key, "second", clientClaims))
	if err != nil || principal.Role != "CLIENT" {
		t.Fatalf("valid client token must reach role check: principal=%+v, err=%v", principal, err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), signToken(t, other, "second", claims)); err == nil {
		t.Fatal("invalid signature accepted")
	}
	available.Store(false)
	verifier.expires = time.Now().Add(-time.Second)
	if _, err := verifier.Verify(t.Context(), token); err == nil {
		t.Fatal("unavailable JWKS accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	hash := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
