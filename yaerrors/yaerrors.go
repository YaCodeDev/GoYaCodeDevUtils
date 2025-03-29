package yaerrors

import (
	"errors"
	"fmt"
	"net/http"
)

type Error interface {
	error
	Wrap(msg string) Error
	Code() int
	Error() string
	Unwrap() error
}

type yaError struct {
	code      int
	cause     error
	traceback string
}

func FromError(code int, cause error, wrap string) Error {
	return &yaError{
		code:      code,
		cause:     cause,
		traceback: fmt.Sprintf("%s: %v", wrap, cause),
	}
}

func FromString(code int, msg string) Error {
	return &yaError{
		code:      code,
		cause:     errors.New(msg), // nolint:err113
		traceback: msg,
	}
}

func (e *yaError) Error() string {
	safetyCheck(&e)

	return fmt.Sprintf("%d | %s", e.code, e.traceback)
}

func (e *yaError) Unwrap() error {
	safetyCheck(&e)

	return e.cause
}

func (e *yaError) Wrap(msg string) Error {
	safetyCheck(&e)
	e.traceback = fmt.Sprintf("%s -> %s", msg, e.traceback)

	return e
}

func (e *yaError) Code() int {
	safetyCheck(&e)

	return e.code
}

func safetyCheck(err **yaError) {
	if *err == nil {
		*err = &yaError{
			code:      http.StatusTeapot,
			cause:     ErrTeapot,
			traceback: ErrTeapot.Error(),
		}
	}
}
