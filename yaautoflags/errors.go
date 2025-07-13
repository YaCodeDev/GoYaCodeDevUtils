package yaautoflags

import (
	"errors"
)

var (
	ErrInstanceNil            = errors.New("instance cannot be nil")
	ErrInstanceNotStruct      = errors.New("instance must be a struct")
	ErrFlagsFieldNotFound     = errors.New("flags field not found in struct")
	ErrFlagsFieldTypeMismatch = errors.New(
		"flags field must be of type uint64, uint32, uint16, uint8, uint or uintptr",
	)
	ErrTooManyFlags = errors.New("too many flags set")
)
