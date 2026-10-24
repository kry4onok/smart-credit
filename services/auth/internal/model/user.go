package model

import "github.com/google/uuid"

type Organization struct {
	ID   uuid.UUID
	INN  string
	Name string
}

// CurrentUser contains no password hash or session credentials.
type CurrentUser struct {
	ID           uuid.UUID
	Email        string
	FullName     string
	Role         string
	CanSubmit    bool
	Organization *Organization
	Active       bool
}
