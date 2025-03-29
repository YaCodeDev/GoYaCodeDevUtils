package valueparser

import (
	"errors"
)

var (
	ErrUnknownType     = errors.New("unknown type")
	ErrInvalidValue    = errors.New("invalid value")
	ErrInvalidType     = errors.New("invalid type")
	ErrUnparsableValue = errors.New("unparsable value")
)
