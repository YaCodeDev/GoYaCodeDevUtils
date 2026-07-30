package yascheduler

import "errors"

var (
	// ErrNilRegistry reports a nil registry passed to a constructor or
	// registration call.
	ErrNilRegistry = errors.New("registry is nil")

	// ErrNilConfig reports a nil configuration passed to a constructor.
	ErrNilConfig = errors.New("config is nil")

	// ErrEmptyFunctionName reports a registration without a function
	// name.
	ErrEmptyFunctionName = errors.New("function name is empty")

	// ErrNilFunction reports a registration with a nil function value.
	ErrNilFunction = errors.New("function is nil")

	// ErrDuplicateFunction reports a second registration under one name
	// and version pair.
	ErrDuplicateFunction = errors.New("function already registered")

	// ErrUnsupportedValueKind reports an argument or result type that
	// MessagePack cannot round-trip.
	ErrUnsupportedValueKind = errors.New("unsupported argument or result kind")

	// ErrEmptyAddress reports a configuration without a scheduler
	// address.
	ErrEmptyAddress = errors.New("scheduler address is empty")

	// ErrEmptyExecutorType reports a configuration without an executor
	// type.
	ErrEmptyExecutorType = errors.New("executor type is empty")

	// ErrClientAlreadyRunning reports a second concurrent Run call.
	ErrClientAlreadyRunning = errors.New("client is already running")

	// ErrNotConnected reports an operation that needs a registered
	// connection while the client has none.
	ErrNotConnected = errors.New("client is not connected")

	// ErrRegistrationRejected reports a scheduler that refused this
	// executor's registration.
	ErrRegistrationRejected = errors.New("registration rejected")

	// ErrConnectionClosed reports a connection that ended while a
	// response was still awaited.
	ErrConnectionClosed = errors.New("connection closed")

	// ErrServerShutdown reports a scheduler that announced a graceful
	// shutdown.
	ErrServerShutdown = errors.New("scheduler is shutting down")

	// ErrUpsertRejected reports a job upsert the scheduler refused.
	ErrUpsertRejected = errors.New("job upsert rejected")

	// ErrUnexpectedMessage reports a frame that is invalid for the
	// current connection state.
	ErrUnexpectedMessage = errors.New("unexpected message")

	// ErrOutgoingQueueFull reports a full outgoing frame queue.
	ErrOutgoingQueueFull = errors.New("outgoing queue is full")

	// ErrEmptyJobKey reports a job spec without a key.
	ErrEmptyJobKey = errors.New("job key is empty")

	// ErrNilJobSpec reports a nil job spec.
	ErrNilJobSpec = errors.New("job spec is nil")
)
