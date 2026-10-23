package model

import "github.com/google/uuid"

type Principal struct {
	UserID    uuid.UUID
	OrgID     uuid.UUID
	Role      string
	CanSubmit bool
}
