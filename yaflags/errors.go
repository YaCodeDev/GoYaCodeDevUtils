package yaflags

import "errors"

var (
	ErrTooManyBits        = errors.New("too many bits for target type")
	ErrBitIndexOutOfRange = errors.New("bit index out of range")
)
