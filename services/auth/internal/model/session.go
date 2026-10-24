package model

import (
	"time"

	"github.com/google/uuid"
)

const RefreshLifetime = 7 * 24 * time.Hour

type User struct {
	Principal
	PasswordHash string
	Active       bool
}

type LoginParams struct{ Email, Password string }

type RefreshSecret struct {
	Token string
	Hash  [32]byte
}

// RefreshSession is the persisted session; it never contains the raw token.
type RefreshSession struct {
	Hash                 [32]byte
	UserID               uuid.UUID
	CreatedAt, ExpiresAt time.Time
}

type SessionState struct {
	Principal
	Active               bool
	CreatedAt, ExpiresAt time.Time
	RevokedAt            *time.Time
}

type LoginResult struct {
	AccessToken      AccessToken
	RefreshToken     string
	RefreshExpiresAt time.Time
}
