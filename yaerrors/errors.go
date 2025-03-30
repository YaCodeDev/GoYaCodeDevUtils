package yaerrors

import "errors"

// ErrTeapot is a custom error to report that the backend developer is a teapot, because
// they are dereferencing a nil error.
// This error is used as a safety measure to prevent nil pointer dereference.
var ErrTeapot = errors.New("backend developer is a teapot")
