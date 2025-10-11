package yaerrors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

func TestYaErrorFromString(t *testing.T) {
	err := yaerrors.FromString(404, "Not Found")
	if err == nil {
		t.Fatalf("Error is nil, got: %v", err)
	}
}

func TestYaErrorFromString_Code(t *testing.T) {
	err := yaerrors.FromString(404, "Not Found")
	if err.Code() != 404 {
		t.Fatalf("Error code is not 404, got: %v", err.Code())
	}
}

func TestYaErrorFromString_Error(t *testing.T) {
	err := yaerrors.FromString(404, "Not Found")
	if err.Error() != "404 | Not Found" {
		t.Fatalf("Error message is not '404 | Not Found', got: %v", err.Error())
	}
}

func TestYaErrorFromError(t *testing.T) {
	err := yaerrors.FromError(404, nil, "Not Found")
	if err == nil {
		t.Fatalf("Error is nil, got: %v", err)
	}
}

func TestYaErrorFromError_Code(t *testing.T) {
	err := yaerrors.FromError(404, nil, "Not Found")
	if err.Code() != 404 {
		t.Fatalf("Error code is not 404, got: %v", err.Code())
	}
}

func TestYaErrorFromError_Error(t *testing.T) {
	err := yaerrors.FromError(404, yaerrors.ErrTeapot, "Not Found")
	if err.Error() != "404 | Not Found: backend developer is a teapot" {
		t.Fatalf(
			"Error message is not '404 | Not Found: backend developer is a teapot', got: %v",
			err.Error(),
		)
	}
}

func TestYaError_Wrap(t *testing.T) {
	err := yaerrors.FromString(404, "Not Found")

	wrappedErr := err.Wrap("Not Found 2")
	if wrappedErr.Error() == "404 | Not Found 2 -> Not Found: New Error 2" {
		t.Fatalf(
			"Wrapped error message is not '404 | Not Found 2 -> Not Found: New Error 2', got: %v",
			wrappedErr.Error(),
		)
	}
}

func TestYaErrorUnwrap_Works(t *testing.T) {
	err := yaerrors.FromError(404, yaerrors.ErrTeapot, "Not Found")
	if !errors.Is(err.Unwrap(), yaerrors.ErrTeapot) {
		t.Fatalf(
			fmt.Sprintf("Error didn't unwrap as %v", yaerrors.ErrTeapot),
			err.Error(),
		)
	}
}

func TestYaErrorUnwrapLastError_Works(t *testing.T) {
	expected := "Wrapped error"

	err := yaerrors.FromError(404, yaerrors.ErrTeapot, "Not Found").Wrap(expected)
	got := err.UnwrapLastError()

	if got != expected {
		t.Fatalf("Error didn't unwrap correctly:\n got: %v\n want: %v", got, expected)
	}
}
