package yascheduler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

type addArgs struct {
	A int
	B int
}

type addResult struct {
	Sum int
}

const (
	testFunctionName    protocol.FunctionName    = "add"
	testFunctionVersion protocol.FunctionVersion = "v1"
)

func addFunction(_ context.Context, args addArgs) (addResult, error) {
	return addResult{Sum: args.A + args.B}, nil
}

func TestRegisterFunctionSuccess(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	if err := yascheduler.RegisterFunction(
		registry,
		testFunctionName,
		testFunctionVersion,
		addFunction,
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	if registry.Len() != 1 {
		t.Fatalf("registry length = %d, want 1", registry.Len())
	}
}

func TestRegisterFunctionEmptyName(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	err := yascheduler.RegisterFunction(registry, "", testFunctionVersion, addFunction)
	if err == nil || !errors.Is(err, yascheduler.ErrEmptyFunctionName) {
		t.Fatalf("err = %v, want ErrEmptyFunctionName", err)
	}
}

func TestRegisterFunctionNilFunction(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	err := yascheduler.RegisterFunction[addArgs, addResult](
		registry,
		testFunctionName,
		testFunctionVersion,
		nil,
	)
	if err == nil || !errors.Is(err, yascheduler.ErrNilFunction) {
		t.Fatalf("err = %v, want ErrNilFunction", err)
	}
}

func TestRegisterFunctionDuplicate(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	if err := yascheduler.RegisterFunction(
		registry,
		testFunctionName,
		testFunctionVersion,
		addFunction,
	); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	err := yascheduler.RegisterFunction(
		registry,
		testFunctionName,
		testFunctionVersion,
		addFunction,
	)
	if err == nil || !errors.Is(err, yascheduler.ErrDuplicateFunction) {
		t.Fatalf("err = %v, want ErrDuplicateFunction", err)
	}
}

func TestRegisterFunctionSameNameDifferentVersion(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	if err := yascheduler.RegisterFunction(
		registry,
		testFunctionName,
		testFunctionVersion,
		addFunction,
	); err != nil {
		t.Fatalf("v1 registration failed: %v", err)
	}

	if err := yascheduler.RegisterFunction(
		registry,
		testFunctionName,
		"v2",
		addFunction,
	); err != nil {
		t.Fatalf("v2 registration failed: %v", err)
	}

	if registry.Len() != 2 {
		t.Fatalf("registry length = %d, want 2", registry.Len())
	}
}

func TestRegisterFunctionUnsupportedKind(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	err := yascheduler.RegisterFunction(
		registry,
		testFunctionName,
		testFunctionVersion,
		func(_ context.Context, args func()) (int, error) {
			_ = args

			return 0, nil
		},
	)
	if err == nil || !errors.Is(err, yascheduler.ErrUnsupportedValueKind) {
		t.Fatalf("err = %v, want ErrUnsupportedValueKind", err)
	}
}

func TestNonRetryableMarking(t *testing.T) {
	t.Parallel()

	cause := errors.New("bad input")

	if yascheduler.IsNonRetryable(cause) {
		t.Fatal("plain error reported non-retryable")
	}

	marked := yascheduler.NonRetryable(cause)
	if !yascheduler.IsNonRetryable(marked) {
		t.Fatal("marked error reported retryable")
	}

	if !errors.Is(marked, cause) {
		t.Fatal("marked error lost its cause")
	}

	if yascheduler.NonRetryable(nil) != nil {
		t.Fatal("NonRetryable(nil) is not nil")
	}
}

func mustEncode(t *testing.T, value any) []byte {
	t.Helper()

	encoded, err := yaencoding.EncodeMessagePack(value)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	return encoded
}

func mustDecode[T any](t *testing.T, payload []byte) T {
	t.Helper()

	decoded, err := yaencoding.DecodeMessagePack[T](payload)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	return *decoded
}
