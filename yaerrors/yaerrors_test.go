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

func TestYaError_Is_SatisfiesStdlibErrorsIs(t *testing.T) {
	err := yaerrors.FromError(404, yaerrors.ErrTeapot, "Not Found")

	if !errors.Is(err, yaerrors.ErrTeapot) {
		t.Fatalf("errors.Is(err, yaerrors.ErrTeapot) = false, want true")
	}
}

func TestYaError_Is_MatchesSameCodeAndCause(t *testing.T) {
	first := yaerrors.FromError(404, yaerrors.ErrTeapot, "first context")
	second := yaerrors.FromError(404, yaerrors.ErrTeapot, "second context")

	if !first.Is(second) {
		t.Fatalf("first.Is(second) = false, want true")
	}
}

func TestYaError_Is_DifferentCodeDoesNotMatch(t *testing.T) {
	first := yaerrors.FromError(404, yaerrors.ErrTeapot, "context")
	second := yaerrors.FromError(500, yaerrors.ErrTeapot, "context")

	if first.Is(second) {
		t.Fatalf("first.Is(second) = true, want false")
	}
}

func TestYaError_Is_DifferentCauseDoesNotMatch(t *testing.T) {
	other := errors.New("other cause")

	first := yaerrors.FromError(404, yaerrors.ErrTeapot, "context")
	second := yaerrors.FromError(404, other, "context")

	if first.Is(second) {
		t.Fatalf("first.Is(second) = true, want false")
	}
}

func TestYaError_Is_NilTargetDoesNotMatch(t *testing.T) {
	err := yaerrors.FromError(404, yaerrors.ErrTeapot, "context")

	if err.Is(nil) {
		t.Fatalf("err.Is(nil) = true, want false")
	}
}

func TestYaError_IsError_MatchesCause(t *testing.T) {
	err := yaerrors.FromError(404, yaerrors.ErrTeapot, "context")

	if !err.IsError(yaerrors.ErrTeapot) {
		t.Fatalf("err.IsError(yaerrors.ErrTeapot) = false, want true")
	}
}

func TestYaError_IsError_NoMatch(t *testing.T) {
	other := errors.New("other cause")

	err := yaerrors.FromError(404, yaerrors.ErrTeapot, "context")

	if err.IsError(other) {
		t.Fatalf("err.IsError(other) = true, want false")
	}
}
