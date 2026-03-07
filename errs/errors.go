package errs

import "errors"

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrUniversityNotFound = errors.New("university not found")
	ErrUnauthorized       = errors.New("unauthorized")
)
