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

	// ErrDrainTimeout reports running functions that outlived both the
	// shutdown drain and the cancellation that followed it.
	ErrDrainTimeout = errors.New("running functions outlived the drain")

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

	// ErrDeleteRejected reports a job delete the scheduler refused.
	ErrDeleteRejected = errors.New("job delete rejected")

	// ErrUnexpectedMessage reports a frame that is invalid for the
	// current connection state.
	ErrUnexpectedMessage = errors.New("unexpected message")

	// ErrOutgoingQueueFull reports a full outgoing frame queue.
	ErrOutgoingQueueFull = errors.New("outgoing queue is full")

	// ErrNilJobSpec reports a nil job spec.
	ErrNilJobSpec = errors.New("job spec is nil")

	// ErrEmptyLabel reports a routing label with no name, which names no
	// routing target and is already what an unpinned job means.
	ErrEmptyLabel = errors.New("routing label is empty")

	// ErrLabelUpdateRejected reports a label update the scheduling engine
	// refused, leaving the announced label set untouched.
	ErrLabelUpdateRejected = errors.New("label update rejected")

	// ErrResultNotRequested reports an Await on a submission whose job was
	// upserted with protocol.ResultModeIgnore, so no result was ever going
	// to arrive.
	ErrResultNotRequested = errors.New("job result delivery was not requested")

	// ErrResultHasNoValue reports a decode of a result that carries no
	// value: a Void function, or an execution that failed before producing
	// one.
	ErrResultHasNoValue = errors.New("result carries no value")

	// ErrSubmissionClosed reports an Await on a submission whose waiter
	// was already released by Close or a previous Await.
	ErrSubmissionClosed = errors.New("submission is closed")

	// ErrNilResult reports a nil result passed to DecodeResult.
	ErrNilResult = errors.New("result is nil")

	// ErrLocalAlreadyRunning reports a second concurrent Run call on one
	// local scheduler.
	ErrLocalAlreadyRunning = errors.New("local scheduler is already running")

	// ErrLoopbackStopped reports a message offered to a loopback whose
	// drain goroutines have been stopped.
	ErrLoopbackStopped = errors.New("loopback is stopped")

	// ErrHeartbeatPumpStopped reports a local heartbeat pump that died
	// while the scheduler was still serving. Silent heartbeat loss would
	// let lease reaping redispatch work still running in this process, so
	// the death fails Run instead of being logged and survived.
	ErrHeartbeatPumpStopped = errors.New("heartbeat pump stopped unexpectedly")
)
