package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

func newManager(t *testing.T) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private.pem")
	if err := GenerateKey(path); err != nil {
		t.Fatal(err)
	}
	m, err := New(config.JWTConfig{Issuer: "smart-credit-auth", Audience: "smart-credit", AccessTTL: 15 * time.Minute,
		KeyID: "auth-v1", PrivateKeyPath: path})
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return time.Unix(1800000000, 0) }
	return m
}

func TestIssueAndVerify(t *testing.T) {
	m := newManager(t)
	for _, p := range []model.Principal{
		{UserID: uuid.New(), OrgID: uuid.New(), Role: model.RoleClient, CanSubmit: true},
		{UserID: uuid.New(), OrgID: uuid.New(), Role: model.RoleClient},
		{UserID: uuid.New(), Role: model.RoleAnalyst},
		{UserID: uuid.New(), Role: model.RoleAdmin},
	} {
		t.Run(p.Role+"/"+p.UserID.String(), func(t *testing.T) {
			token, err := m.Issue(p)
			if err != nil {
				t.Fatal(err)
			}
			if token.ExpiresIn != 900 {
				t.Fatalf("expires_in=%d", token.ExpiresIn)
			}
			actual, err := m.Verify(t.Context(), token.Token)
			if err != nil || actual != p {
				t.Fatalf("principal=%+v error=%v", actual, err)
			}
			parts := strings.Split(token.Token, ".")
			var h map[string]any
			var c map[string]any
			if decode(parts[0], &h) != nil || decode(parts[1], &c) != nil {
				t.Fatal("invalid token encoding")
			}
			if h["alg"] != "RS256" || h["kid"] != "auth-v1" || h["typ"] != "JWT" || c["sub"] != p.UserID.String() ||
				c["iss"] != m.cfg.Issuer || c["aud"] != m.cfg.Audience || c["exp"] != float64(1800000900) || c["can_submit"] != p.CanSubmit {
				t.Fatal("access token contract mismatch")
			}
			if _, hasOrg := c["org_id"]; hasOrg != (p.Role == model.RoleClient) {
				t.Fatal("org_id must occur only in CLIENT tokens")
			}
			// Verify independently using the public JWKS parameters.
			jwk := m.PublicKey()
			n, nErr := base64.RawURLEncoding.DecodeString(jwk.N)
			e, eErr := base64.RawURLEncoding.DecodeString(jwk.E)
			if nErr != nil || eErr != nil || jwk.KeyID != h["kid"] {
				t.Fatal("invalid JWK")
			}
			key := rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
			signature, err := base64.RawURLEncoding.DecodeString(parts[2])
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			if err := rsa.VerifyPKCS1v15(&key, crypto.SHA256, digest[:], signature); err != nil {
				t.Fatal("JWK cannot verify issued token:", err)
			}
		})
	}
}

func TestIssueRejectsInvalidPrincipal(t *testing.T) {
	m := newManager(t)
	for _, p := range []model.Principal{
		{Role: model.RoleClient, OrgID: uuid.New()},
		{UserID: uuid.New(), Role: model.RoleClient},
		{UserID: uuid.New(), Role: "UNKNOWN"},
		{UserID: uuid.New(), Role: model.RoleAdmin, OrgID: uuid.New()},
		{UserID: uuid.New(), Role: model.RoleAnalyst, CanSubmit: true},
	} {
		if token, err := m.Issue(p); err == nil || token.Token != "" {
			t.Fatal("invalid principal accepted")
		}
	}
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	m := newManager(t)
	for _, tc := range []struct {
		name   string
		mutate func(*header, *claims)
	}{
		{"none", func(h *header, _ *claims) { h.Algorithm = "none" }},
		{"HS256", func(h *header, _ *claims) { h.Algorithm = "HS256" }},
		{"unknown kid", func(h *header, _ *claims) { h.KeyID = "other-key" }},
		{"missing kid", func(h *header, _ *claims) { h.KeyID = "" }},
		{"critical header", func(h *header, _ *claims) { h.Critical = []string{"custom"} }},
		{"wrong type", func(h *header, _ *claims) { h.Type = "refresh" }},
		{"issuer", func(_ *header, c *claims) { c.Issuer = "another-auth" }},
		{"audience", func(_ *header, c *claims) { c.Audience = "another-service" }},
		{"expired at boundary", func(_ *header, c *claims) { c.Expires = 1800000000 }},
		{"missing expiration", func(_ *header, c *claims) { c.Expires = 0 }},
		{"not yet valid", func(_ *header, c *claims) { c.NotBefore = 1800000001 }},
		{"future issue", func(_ *header, c *claims) { c.IssuedAt = 1800000001 }},
		{"missing subject", func(_ *header, c *claims) { c.Subject = "" }},
		{"nil subject", func(_ *header, c *claims) { c.Subject = uuid.Nil.String() }},
		{"bad subject", func(_ *header, c *claims) { c.Subject = "bad" }},
		{"missing org", func(_ *header, c *claims) { c.OrgID = "" }},
		{"nil org", func(_ *header, c *claims) { c.OrgID = uuid.Nil.String() }},
		{"unknown role", func(_ *header, c *claims) { c.Role = "UNKNOWN" }},
		{"staff org", func(_ *header, c *claims) { c.Role = model.RoleAdmin; c.CanSubmit = false }},
		{"staff submit", func(_ *header, c *claims) { c.Role = model.RoleAnalyst; c.OrgID = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := header{Algorithm: "RS256", Type: "JWT", KeyID: m.cfg.KeyID}
			c := claims{Subject: uuid.NewString(), OrgID: uuid.NewString(), Role: model.RoleClient, CanSubmit: true,
				Issuer: m.cfg.Issuer, Audience: m.cfg.Audience, IssuedAt: 1800000000, Expires: 1800000900}
			tc.mutate(&h, &c)
			assertInvalid(t, m, signedToken(t, m.key, h, c))
		})
	}
	valid, err := m.Issue(model.Principal{UserID: uuid.New(), Role: model.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid.Token, ".")
	for _, raw := range []string{"", "not-a-token", "a.b.c", strings.Repeat("x", 16385),
		parts[0] + "." + parts[1] + ".", parts[0] + "." + parts[1] + ".AAAA",
		parts[0] + "." + parts[1] + "A." + parts[2]} {
		assertInvalid(t, m, raw)
	}
	other := newManager(t)
	assertInvalid(t, other, valid.Token)
	m.now = func() time.Time { return time.Unix(1800000900, 0) }
	assertInvalid(t, m, valid.Token)
}

func assertInvalid(t *testing.T, m *Manager, token string) {
	t.Helper()
	if p, err := m.Verify(t.Context(), token); !errors.Is(err, ErrInvalidToken) || p != (model.Principal{}) {
		t.Fatalf("invalid token accepted: principal=%+v, error=%v", p, err)
	}
}

func signedToken(t *testing.T, key *rsa.PrivateKey, h header, c claims) string {
	t.Helper()
	hData, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	cData, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(hData) + "." + base64.RawURLEncoding.EncodeToString(cData)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestReloadKeepsTokensValid(t *testing.T) {
	m := newManager(t)
	token, err := m.Issue(model.Principal{UserID: uuid.New(), Role: model.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := New(m.cfg)
	if err != nil {
		t.Fatal(err)
	}
	reloaded.now = m.now
	if reloaded.PublicKey() != m.PublicKey() {
		t.Fatal("public key changed on restart")
	}
	if _, err := reloaded.Verify(t.Context(), token.Token); err != nil {
		t.Fatal("token became invalid on restart:", err)
	}
}
