package domain

import "errors"

var (
	ErrCompanyNotFound        = errors.New("company not found")
	ErrCompanyAlreadyExists   = errors.New("company already exists")
	ErrCompanyVersionConflict = errors.New("company version conflict")
	ErrInvalidCompanyProfile  = errors.New("invalid company profile")
	ErrInvalidIfMatch         = errors.New("invalid if-match")
)
