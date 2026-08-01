package yascheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// InvokeFunc is a prepared invocation wrapper. It decodes raw MessagePack
// arguments, calls the registered function, and encodes its result. A
// non-nil WireError describes a structured execution failure instead of a
// Go error so the classification travels to the scheduler unchanged.
type InvokeFunc func(ctx context.Context, args []byte) ([]byte, *protocol.WireError)

type registryKey struct {
	name    protocol.FunctionName
	version protocol.FunctionVersion
}

type preparedFunction struct {
	spec   protocol.FunctionSpec
	invoke InvokeFunc
}

// Registry holds the functions one executor process can run. Register
// every function before starting the Client; the registry is safe for
// concurrent use, but functions registered after the Client connected are
// only advertised on the next reconnect.
type Registry struct {
	mu        sync.RWMutex
	functions map[registryKey]*preparedFunction
	order     []registryKey
}

// NewRegistry returns an empty function registry.
func NewRegistry() *Registry {
	return &Registry{
		functions: make(map[registryKey]*preparedFunction),
	}
}

// RegisterFunction registers fn under the given name and version. The
// argument type A and result type R are validated at registration time
// and their signatures are derived once and cached; execution decodes
// arguments with MessagePack, calls fn, and encodes the result with
// MessagePack. fn errors are retryable by default; wrap an error with
// NonRetryable to consume no further retries. A panic inside fn is
// recovered and reported as a structured execution error.
func RegisterFunction[A any, R any](
	registry *Registry,
	name protocol.FunctionName,
	version protocol.FunctionVersion,
	fn func(ctx context.Context, args A) (R, error),
) yaerrors.Error {
	if registry == nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilRegistry,
			logTag+" register function",
		)
	}

	if name == "" {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyFunctionName,
			logTag+" register function",
		)
	}

	if fn == nil {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrNilFunction,
			logTag+" register function",
		)
	}

	argType := reflect.TypeFor[A]()
	if err := validateValueType(argType); err != nil {
		return err.Wrap(logTag + " register function: argument type")
	}

	resultType := reflect.TypeFor[R]()
	if err := validateValueType(resultType); err != nil {
		return err.Wrap(logTag + " register function: result type")
	}

	prepared := &preparedFunction{
		spec: protocol.FunctionSpec{
			Name:            name,
			Version:         version,
			InputSignature:  argType.String(),
			OutputSignature: resultType.String(),
		},
		invoke: prepareInvoker(fn),
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	key := registryKey{name: name, version: version}
	if _, exists := registry.functions[key]; exists {
		return yaerrors.FromError(
			http.StatusConflict,
			ErrDuplicateFunction,
			fmt.Sprintf(logTag+" register function %q version %q", name, version),
		)
	}

	if len(registry.order) >= int(protocol.DefaultMaxFunctions) {
		return yaerrors.FromError(
			http.StatusBadRequest,
			protocol.ErrTooManyFunctions,
			logTag+" register function",
		)
	}

	registry.functions[key] = prepared
	registry.order = append(registry.order, key)

	return nil
}

// prepareInvoker builds the registration-time invocation wrapper: decode
// arguments, call the typed function, encode the result, recover panics.
// A Void result type is detected here, once, so such a function reports no
// value instead of an encoded empty struct.
func prepareInvoker[A any, R any](
	fn func(ctx context.Context, args A) (R, error),
) InvokeFunc {
	voidResult := reflect.TypeFor[R]() == reflect.TypeFor[Void]()

	return func(ctx context.Context, raw []byte) (out []byte, wireErr *protocol.WireError) {
		defer func() {
			if reason := recover(); reason != nil {
				out = nil
				wireErr = &protocol.WireError{
					Code:      protocol.ErrorCodeFunctionPanic,
					Retryable: true,
					Message:   fmt.Sprintf("function panicked: %v", reason),
				}
			}
		}()

		args, decodeErr := yaencoding.DecodeMessagePack[A](raw)
		if decodeErr != nil {
			return nil, &protocol.WireError{
				Code:      protocol.ErrorCodeInvalidArguments,
				Retryable: false,
				Message:   decodeErr.UnwrapLastError(),
			}
		}

		result, fnErr := fn(ctx, *args)
		if fnErr != nil {
			return nil, &protocol.WireError{
				Code:      protocol.ErrorCodeFunctionError,
				Retryable: !IsNonRetryable(fnErr),
				Message:   fnErr.Error(),
			}
		}

		if voidResult {
			return nil, nil
		}

		payload, encodeErr := yaencoding.EncodeMessagePack(result)
		if encodeErr != nil {
			return nil, &protocol.WireError{
				Code:      protocol.ErrorCodeInternal,
				Retryable: false,
				Message:   encodeErr.UnwrapLastError(),
			}
		}

		return payload, nil
	}
}

// validateValueType rejects types MessagePack cannot round-trip, so an
// unsupported function shape fails at startup instead of at dispatch.
func validateValueType(t reflect.Type) yaerrors.Error {
	switch t.Kind() {
	case reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Uintptr:
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrUnsupportedValueKind,
			fmt.Sprintf("kind %s", t.Kind()),
		)
	case reflect.Invalid,
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.Array, reflect.Interface, reflect.Map, reflect.Pointer,
		reflect.Slice, reflect.String, reflect.Struct:
		return nil
	default:
		return nil
	}
}

// specs snapshots the advertised function specs in registration order.
func (r *Registry) specs() []protocol.FunctionSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	specs := make([]protocol.FunctionSpec, 0, len(r.order))
	for _, key := range r.order {
		specs = append(specs, r.functions[key].spec)
	}

	return specs
}

// lookup returns the prepared function registered under name and version.
func (r *Registry) lookup(
	name protocol.FunctionName,
	version protocol.FunctionVersion,
) (*preparedFunction, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	prepared, found := r.functions[registryKey{name: name, version: version}]

	return prepared, found
}

// Len reports how many functions are registered.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.order)
}

type nonRetryableError struct {
	cause error
}

func (e *nonRetryableError) Error() string {
	return e.cause.Error()
}

func (e *nonRetryableError) Unwrap() error {
	return e.cause
}

// NonRetryable marks err as consuming no function retries: the scheduler
// fails the execution permanently on this attempt.
func NonRetryable(err error) error {
	if err == nil {
		return nil
	}

	return &nonRetryableError{cause: err}
}

// IsNonRetryable reports whether err (or any error it wraps) was marked
// with NonRetryable.
func IsNonRetryable(err error) bool {
	var marker *nonRetryableError

	return errors.As(err, &marker)
}
