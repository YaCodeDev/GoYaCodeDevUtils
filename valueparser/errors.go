package valueparser

import (
	"errors"
)

var (
	ErrUnknownType       = errors.New("unknown type")
	ErrInvalidValue      = errors.New("invalid value")
	ErrUnconvertibleType = errors.New("unconvertible type")
	ErrInvalidType       = errors.New("invalid type")
	ErrUnparsableValue   = errors.New("unparsable value")
	ErrInvalidEntry      = errors.New("invalid entry")
)
