package model

import "github.com/google/uuid"

const (
	RoleClient  = "CLIENT"
	RoleAnalyst = "ANALYST"
	RoleAdmin   = "ADMIN"
)

type Principal struct {
	UserID    uuid.UUID
	OrgID     uuid.UUID
	Role      string
	CanSubmit bool
}

type AccessToken struct {
	Token     string
	ExpiresIn int
}

// PublicKey contains only the public RSA parameters of a signing key.
type PublicKey struct {
	KeyID string
	N     string
	E     string
}
