package domain

import "errors"

var (
	ErrUnauthorized   = errors.New("invalid credentials or session")
	ErrInvalidRequest = errors.New("invalid request")
	ErrUserNotFound   = errors.New("user not found")
	ErrUnavailable    = errors.New("temporary dependency failure")
)
