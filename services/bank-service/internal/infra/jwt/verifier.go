package jwt

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/config"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid access token")

type Principal struct {
	UserID uuid.UUID
	Role   string
}

type Verifier struct {
	url      string
	issuer   string
	audience string
	client   *http.Client
	ttl      time.Duration
	mu       sync.Mutex
	keys     map[string]*rsa.PublicKey
	expires  time.Time
	// lastUnknownRefresh rate-limits refreshes caused by an unknown kid, so forged
	// tokens cannot make every request wait for a JWKS download.
	lastUnknownRefresh time.Time
}

const unknownKidRefreshInterval = 10 * time.Second

func New(cfg config.JWTConfig) *Verifier {
	return &Verifier{url: cfg.JWKSURL, issuer: cfg.Issuer, audience: cfg.Audience,
		client: &http.Client{Timeout: cfg.Timeout}, ttl: cfg.CacheTTL}
}

type claims struct {
	Subject   string          `json:"sub"`
	OrgID     string          `json:"org_id"`
	Role      string          `json:"role"`
	Issuer    string          `json:"iss"`
	Audience  json.RawMessage `json:"aud"`
	IssuedAt  int64           `json:"iat"`
	Expires   int64           `json:"exp"`
	NotBefore int64           `json:"nbf"`
}

func (v *Verifier) Verify(ctx context.Context, token string) (Principal, error) {
	if len(token) > 16384 || v.issuer == "" || v.audience == "" {
		return Principal{}, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Principal{}, ErrInvalidToken
	}
	var header struct {
		Algorithm string   `json:"alg"`
		Type      string   `json:"typ"`
		KeyID     string   `json:"kid"`
		Critical  []string `json:"crit"`
	}
	if decode(parts[0], &header) != nil || header.Algorithm != "RS256" ||
		header.Type != "JWT" || header.KeyID == "" || len(header.Critical) != 0 {
		return Principal{}, ErrInvalidToken
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	key, err := v.key(ctx, header.KeyID)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return Principal{}, ErrInvalidToken
	}
	var c claims
	now := time.Now().Unix()
	if decode(parts[1], &c) != nil || c.Issuer != v.issuer || !hasAudience(c.Audience, v.audience) ||
		c.IssuedAt <= 0 || c.IssuedAt > now || c.Expires <= now || c.Expires <= c.IssuedAt || c.NotBefore > now ||
		(c.Role != "CLIENT" && c.Role != "ANALYST" && c.Role != "ADMIN") {
		return Principal{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(c.Subject)
	if err != nil || userID == uuid.Nil {
		return Principal{}, ErrInvalidToken
	}
	if c.Role == "CLIENT" {
		orgID, err := uuid.Parse(c.OrgID)
		if err != nil || orgID == uuid.Nil {
			return Principal{}, ErrInvalidToken
		}
	} else if c.OrgID != "" {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: userID, Role: c.Role}, nil
}

func decode(part string, target any) error {
	data, err := base64.RawURLEncoding.Strict().DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func hasAudience(raw json.RawMessage, audience string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == audience
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		for _, item := range list {
			if item == audience {
				return true
			}
		}
	}
	return false
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := time.Now()
	if key := v.keys[kid]; key != nil && now.Before(v.expires) {
		return key, nil
	}
	if v.keys != nil && now.Before(v.expires) {
		if now.Sub(v.lastUnknownRefresh) < unknownKidRefreshInterval {
			return nil, ErrInvalidToken
		}
		v.lastUnknownRefresh = now
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}

	response, err := v.client.Do(request)
	if err != nil {
		return nil, ErrInvalidToken
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, ErrInvalidToken
	}
	const maxJWKSBytes = 1 << 20

	data, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSBytes+1))
	if err != nil || len(data) > maxJWKSBytes {
		return nil, ErrInvalidToken
	}
	var document struct {
		Keys []struct {
			KeyID string `json:"kid"`
			Type  string `json:"kty"`
			Use   string `json:"use"`
			Alg   string `json:"alg"`
			N     string `json:"n"`
			E     string `json:"e"`
		} `json:"keys"`
	}
	if json.Unmarshal(data, &document) != nil {
		return nil, ErrInvalidToken
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range document.Keys {
		if jwk.Type != "RSA" || jwk.Alg != "RS256" || jwk.Use != "sig" || jwk.KeyID == "" {
			continue
		}
		n, nErr := base64.RawURLEncoding.Strict().DecodeString(jwk.N)
		e, eErr := base64.RawURLEncoding.Strict().DecodeString(jwk.E)
		if nErr != nil || eErr != nil || len(e) == 0 || len(e) > 4 {
			return nil, ErrInvalidToken
		}
		modulus := new(big.Int).SetBytes(n)
		exponent := new(big.Int).SetBytes(e).Int64()
		if modulus.BitLen() < 2048 || exponent < 3 || exponent > 2147483647 || exponent%2 == 0 || keys[jwk.KeyID] != nil {
			return nil, ErrInvalidToken
		}
		keys[jwk.KeyID] = &rsa.PublicKey{N: modulus, E: int(exponent)}
	}
	v.keys, v.expires = keys, time.Now().Add(v.ttl)
	if key := keys[kid]; key != nil {
		return key, nil
	}
	return nil, ErrInvalidToken
}
