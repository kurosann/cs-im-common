package dataaccess

import (
	"errors"
)

var (
	ErrDataNotFound     = errors.New("data not found")
	ErrIDRequired       = errors.New("id is required")
	ErrRevisionMismatch = errors.New("revision mismatch")
)
