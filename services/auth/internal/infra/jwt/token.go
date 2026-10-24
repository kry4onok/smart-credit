// Package jwt issues and verifies RS256 access tokens using a persistent RSA key.
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
	"math/big"
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/config"
	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/model"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid access token")

type Manager struct {
	key *rsa.PrivateKey
	cfg config.JWTConfig
	now func() time.Time
}

type header struct {
	Algorithm string   `json:"alg"`
	Type      string   `json:"typ"`
	KeyID     string   `json:"kid"`
	Critical  []string `json:"crit,omitempty"`
}

type claims struct {
	Subject   string `json:"sub"`
	OrgID     string `json:"org_id,omitempty"`
	Role      string `json:"role"`
	CanSubmit bool   `json:"can_submit"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	Expires   int64  `json:"exp"`
	NotBefore int64  `json:"nbf,omitempty"`
}

func New(cfg config.JWTConfig) (*Manager, error) {
	if strings.TrimSpace(cfg.Issuer) == "" || strings.TrimSpace(cfg.Audience) == "" || strings.TrimSpace(cfg.KeyID) == "" || cfg.AccessTTL < time.Second {
		return nil, errors.New("invalid JWT configuration")
	}
	key, err := loadKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	return &Manager{key: key, cfg: cfg, now: time.Now}, nil
}

func (m *Manager) Issue(principal model.Principal) (model.AccessToken, error) {
	if !validPrincipal(principal) {
		return model.AccessToken{}, errors.New("invalid access token principal")
	}
	now := m.now().Unix()
	ttl := int64(m.cfg.AccessTTL / time.Second)
	c := claims{Subject: principal.UserID.String(), Role: principal.Role, CanSubmit: principal.CanSubmit,
		Issuer: m.cfg.Issuer, Audience: m.cfg.Audience, IssuedAt: now, Expires: now + ttl}
	if principal.Role == model.RoleClient {
		c.OrgID = principal.OrgID.String()
	}
	h, err := encode(header{Algorithm: "RS256", Type: "JWT", KeyID: m.cfg.KeyID})
	if err != nil {
		return model.AccessToken{}, err
	}
	payload, err := encode(c)
	if err != nil {
		return model.AccessToken{}, err
	}
	unsigned := h + "." + payload
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, m.key, crypto.SHA256, digest[:])
	if err != nil {
		return model.AccessToken{}, errors.New("cannot sign access token")
	}
	return model.AccessToken{Token: unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), ExpiresIn: int(ttl)}, nil
}

func (m *Manager) Verify(_ context.Context, token string) (model.Principal, error) {
	if len(token) > 16384 {
		return model.Principal{}, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return model.Principal{}, ErrInvalidToken
	}
	var h header
	if decode(parts[0], &h) != nil || h.Algorithm != "RS256" || h.Type != "JWT" || h.KeyID != m.cfg.KeyID || len(h.Critical) != 0 {
		return model.Principal{}, ErrInvalidToken
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if err != nil {
		return model.Principal{}, ErrInvalidToken
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(&m.key.PublicKey, crypto.SHA256, digest[:], signature) != nil {
		return model.Principal{}, ErrInvalidToken
	}
	var c claims
	now := m.now().Unix()
	if decode(parts[1], &c) != nil || c.Issuer != m.cfg.Issuer || c.Audience != m.cfg.Audience ||
		c.Expires <= now || c.IssuedAt <= 0 || c.IssuedAt > now || c.Expires <= c.IssuedAt || c.NotBefore > now {
		return model.Principal{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		return model.Principal{}, ErrInvalidToken
	}
	p := model.Principal{UserID: userID, Role: c.Role, CanSubmit: c.CanSubmit}
	if c.OrgID != "" {
		p.OrgID, err = uuid.Parse(c.OrgID)
		if err != nil || p.OrgID == uuid.Nil || c.Role != model.RoleClient {
			return model.Principal{}, ErrInvalidToken
		}
	}
	if !validPrincipal(p) {
		return model.Principal{}, ErrInvalidToken
	}
	return p, nil
}

func (m *Manager) PublicKey() model.PublicKey {
	return model.PublicKey{KeyID: m.cfg.KeyID, N: base64.RawURLEncoding.EncodeToString(m.key.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(m.key.E)).Bytes())}
}

func validPrincipal(p model.Principal) bool {
	if p.UserID == uuid.Nil {
		return false
	}
	switch p.Role {
	case model.RoleClient:
		return p.OrgID != uuid.Nil
	case model.RoleAnalyst, model.RoleAdmin:
		return p.OrgID == uuid.Nil && !p.CanSubmit
	default:
		return false
	}
}

func encode(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", errors.New("cannot encode access token")
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decode(value string, target any) error {
	data, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
