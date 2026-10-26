// Package jwt verifies RS256 access tokens using the auth API's public JWKS.
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

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
)

var errInvalidToken = errors.New("invalid access token")

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
	CanSubmit bool            `json:"can_submit"`
	Issuer    string          `json:"iss"`
	Audience  json.RawMessage `json:"aud"`
	Expires   int64           `json:"exp"`
	NotBefore int64           `json:"nbf"`
}

func (v *Verifier) Verify(ctx context.Context, token string) (model.Principal, error) {
	if len(token) > 16384 || v.issuer == "" || v.audience == "" {
		return model.Principal{}, errInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return model.Principal{}, errInvalidToken
	}
	var header struct {
		Algorithm string   `json:"alg"`
		KeyID     string   `json:"kid"`
		Critical  []string `json:"crit"`
	}
	if decode(parts[0], &header) != nil || header.Algorithm != "RS256" || header.KeyID == "" || len(header.Critical) != 0 {
		return model.Principal{}, errInvalidToken
	}
	var c claims
	if decode(parts[1], &c) != nil || c.Issuer != v.issuer || !hasAudience(c.Audience, v.audience) || c.Expires <= time.Now().Unix() || c.NotBefore > time.Now().Unix() {
		return model.Principal{}, errInvalidToken
	}
	userID, err := uuid.Parse(c.Subject)
	if err != nil || userID == uuid.Nil {
		return model.Principal{}, errInvalidToken
	}
	var orgID uuid.UUID
	if c.Role == "CLIENT" {
		orgID, err = uuid.Parse(c.OrgID)
		if err != nil || orgID == uuid.Nil {
			return model.Principal{}, errInvalidToken
		}
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return model.Principal{}, errInvalidToken
	}
	key, err := v.key(ctx, header.KeyID)
	if err != nil {
		return model.Principal{}, errInvalidToken
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return model.Principal{}, errInvalidToken
	}
	return model.Principal{UserID: userID, OrgID: orgID, Role: c.Role, CanSubmit: c.CanSubmit}, nil
}

func decode(part string, target any) error {
	data, err := base64.RawURLEncoding.DecodeString(part)
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
			return nil, errInvalidToken
		}
		v.lastUnknownRefresh = now
	}
	// Refresh on expiration or an unknown kid, allowing auth key rotation.
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return nil, errInvalidToken
	}
	response, err := v.client.Do(request)
	if err != nil {
		return nil, errInvalidToken
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, errInvalidToken
	}
	const maxJWKSBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSBytes+1))
	if err != nil || len(data) > maxJWKSBytes {
		return nil, errInvalidToken
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
		return nil, errInvalidToken
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, jwk := range document.Keys {
		if jwk.Type != "RSA" || jwk.Alg != "RS256" || jwk.Use != "sig" || jwk.KeyID == "" {
			continue
		}
		n, nErr := base64.RawURLEncoding.DecodeString(jwk.N)
		e, eErr := base64.RawURLEncoding.DecodeString(jwk.E)
		if nErr != nil || eErr != nil || len(e) == 0 || len(e) > 4 {
			return nil, errInvalidToken
		}
		modulus := new(big.Int).SetBytes(n)
		exponent := new(big.Int).SetBytes(e).Int64()
		if modulus.BitLen() < 2048 || exponent < 3 || exponent > 2147483647 || exponent%2 == 0 || keys[jwk.KeyID] != nil {
			return nil, errInvalidToken
		}
		keys[jwk.KeyID] = &rsa.PublicKey{N: modulus, E: int(exponent)}
	}
	v.keys, v.expires = keys, time.Now().Add(v.ttl)
	key := keys[kid]
	if key == nil {
		return nil, errInvalidToken
	}
	return key, nil
}
