package yaerrors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

// Package yaerrors provides a custom error type with additional functionality like
// error codes, wrapping, and unwrapping. It is designed to be used in applications
// where detailed error handling and tracing are required.
// The custom error type implements the standard error interface and provides
// additional methods for error handling and tracing.
// It allows for wrapping errors with additional context and retrieving the original
// error message.
type Error interface {
	error
	Wrap(msg string) Error
	WrapWithLog(msg string, log yalogger.Logger) Error
	Code() int
	Error() string
	Unwrap() error
	UnwrapLastError() string
}

const (
	codeSeparate  = " | "
	errorSeparate = " -> "
)

// Minimal error implementation for Error interface.
type yaError struct {
	code      int
	cause     error
	traceback string
}

// Generates a new Error from an existing error with a custom code and message.
// It wraps the original error with additional context and returns a new Error instance.
func FromError(code int, cause error, wrap string) Error {
	return &yaError{
		code:      code,
		cause:     cause,
		traceback: fmt.Sprintf("%s: %v", wrap, cause),
	}
}

// Generates a new Error from an existing error with a custom code and message.
// It wraps the original error with additional context and returns a new Error instance.
// It also logs the error message using the provided logger.
func FromErrorWithLog(code int, cause error, wrap string, log yalogger.Logger) Error {
	msg := fmt.Sprintf("%s: %v", wrap, cause)
	log.Error(msg)

	return &yaError{
		code:      code,
		cause:     cause,
		traceback: msg,
	}
}

// Generates a new Error from a string message with a custom code.
// It creates a new Error instance with the provided code and message.
func FromString(code int, msg string) Error {
	return &yaError{
		code:      code,
		cause:     errors.New(msg), //nolint:err113
		traceback: msg,
	}
}

// Generates a new Error from a string message with a custom code.
// It creates a new Error instance with the provided code and message.
// It also logs the error message using the provided logger.
func FromStringWithLog(code int, msg string, log yalogger.Logger) Error {
	log.Error(msg)

	return &yaError{
		code:      code,
		cause:     errors.New(msg), //nolint:err113
		traceback: msg,
	}
}

// Returns the error code and traceback message as a string.
func (e *yaError) Error() string {
	safetyCheck(&e)

	return fmt.Sprintf("%d%s%s", e.code, codeSeparate, e.traceback)
}

// Returns the original error that caused this error.
func (e *yaError) Unwrap() error {
	safetyCheck(&e)

	return e.cause
}

// Returns the last error.
func (e *yaError) UnwrapLastError() string {
	safetyCheck(&e)

	traceback := []byte(e.traceback)

	end := strings.Index(e.traceback, errorSeparate)
	if end == -1 {
		return e.traceback
	}

	return string(traceback[:end])
}

// Wrap adds a message to the error traceback, providing additional context.
// It is highly recommended to use this method each time you return the error
// to a higher level in the call stack.
func (e *yaError) Wrap(msg string) Error {
	safetyCheck(&e)
	e.traceback = fmt.Sprintf("%s%s%s", msg, errorSeparate, e.traceback)

	return e
}

// Wrap adds a message to the error traceback, providing additional context.
// It is highly recommended to use this method each time you return the error
// to a higher level in the call stack.
// It also logs the error message using the provided logger.
func (e *yaError) WrapWithLog(msg string, log yalogger.Logger) Error {
	log.Error(msg)

	return e.Wrap(msg)
}

// Code returns the error code associated with this error.
func (e *yaError) Code() int {
	safetyCheck(&e)

	return e.code
}

// safetyCheck is a helper function to ensure memory safety.
// It checks if the error is nil and sets a default error if it is.
// This is a safety measure to prevent nil pointer dereference.
// It sets a default "developer is a teapot" error if the error is nil.
func safetyCheck(err **yaError) {
	if *err == nil {
		*err = &yaError{
			code:      http.StatusTeapot,
			cause:     ErrTeapot,
			traceback: ErrTeapot.Error(),
		}
	}
}
