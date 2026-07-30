package protocol

const (
	// Magic is the 4-byte preamble ("YASC") every yascheduler frame starts with.
	Magic uint32 = 0x59415343

	// Version1 is the first protocol version.
	Version1 uint8 = 1

	// CurrentVersion is the protocol version this package speaks.
	CurrentVersion = Version1

	// HeaderSize is the fixed byte length of an encoded frame header.
	HeaderSize = 20
)

// Default wire limits, applied wherever a Limits field is zero.
const (
	// DefaultMaxFrameSize caps a frame payload at 1 MiB.
	DefaultMaxFrameSize uint32 = 1 << 20

	// DefaultMaxStringLen caps string fields at 4 KiB.
	DefaultMaxStringLen uint32 = 1 << 12

	// DefaultMaxBytesLen caps opaque byte fields at 512 KiB.
	DefaultMaxBytesLen uint32 = 1 << 19

	// DefaultMaxFunctions caps one registration at 1024 functions.
	DefaultMaxFunctions uint32 = 1 << 10
)

// Message types of protocol version 1.
const (
	// MessageTypeRegister carries an executor registration request.
	MessageTypeRegister MessageType = 1

	// MessageTypeRegisterAck answers a registration request.
	MessageTypeRegisterAck MessageType = 2

	// MessageTypeHeartbeat carries an executor liveness report.
	MessageTypeHeartbeat MessageType = 3

	// MessageTypeHeartbeatAck answers a heartbeat.
	MessageTypeHeartbeatAck MessageType = 4

	// MessageTypeExecRequest asks an executor to run a function.
	MessageTypeExecRequest MessageType = 5

	// MessageTypeExecAccept reports whether an executor accepted an
	// execution request.
	MessageTypeExecAccept MessageType = 6

	// MessageTypeExecResult reports the outcome of an accepted execution.
	MessageTypeExecResult MessageType = 7

	// MessageTypeCancel asks an executor to cancel a running execution.
	MessageTypeCancel MessageType = 8

	// MessageTypeProtocolError reports a wire-level failure, including
	// unsupported protocol versions.
	MessageTypeProtocolError MessageType = 9

	// MessageTypeJobUpsert creates or updates a job definition.
	MessageTypeJobUpsert MessageType = 10

	// MessageTypeJobUpsertAck answers a job upsert.
	MessageTypeJobUpsertAck MessageType = 11

	// MessageTypeShutdown announces a graceful connection shutdown.
	MessageTypeShutdown MessageType = 12
)

// Structured wire error codes.
const (
	// ErrorCodeUnsupportedVersion rejects a frame whose version byte the
	// receiver does not speak.
	ErrorCodeUnsupportedVersion ErrorCode = 1

	// ErrorCodeMalformedFrame rejects a frame whose payload failed to
	// decode.
	ErrorCodeMalformedFrame ErrorCode = 2

	// ErrorCodeFrameTooLarge rejects a frame whose declared payload
	// length exceeds the receiver's limit.
	ErrorCodeFrameTooLarge ErrorCode = 3

	// ErrorCodeUnknownMessageType rejects a frame with a message type the
	// receiver does not know.
	ErrorCodeUnknownMessageType ErrorCode = 4

	// ErrorCodeRegistrationRejected reports a refused executor
	// registration.
	ErrorCodeRegistrationRejected ErrorCode = 5

	// ErrorCodeUnknownFunction reports an execution request for a
	// function the executor does not have registered.
	ErrorCodeUnknownFunction ErrorCode = 6

	// ErrorCodeIncompatibleFunction reports an execution request whose
	// function version or signatures do not match the registration.
	ErrorCodeIncompatibleFunction ErrorCode = 7

	// ErrorCodeCapacityExhausted reports an executor that cannot accept
	// more concurrent executions.
	ErrorCodeCapacityExhausted ErrorCode = 8

	// ErrorCodeInvalidArguments reports argument payloads the executor
	// could not decode for the target function.
	ErrorCodeInvalidArguments ErrorCode = 9

	// ErrorCodeFunctionError reports a function that ran and returned an
	// error.
	ErrorCodeFunctionError ErrorCode = 10

	// ErrorCodeFunctionPanic reports a function invocation that panicked
	// and was recovered by the executor.
	ErrorCodeFunctionPanic ErrorCode = 11

	// ErrorCodeShuttingDown reports an operation refused because the
	// peer is draining for shutdown.
	ErrorCodeShuttingDown ErrorCode = 12

	// ErrorCodeInternal reports an unclassified internal failure.
	ErrorCodeInternal ErrorCode = 13

	// ErrorCodeExecutionCancelled reports an execution that stopped
	// because the scheduler cancelled it.
	ErrorCodeExecutionCancelled ErrorCode = 14
)

// Schedule kinds.
const (
	// ScheduleKindOneShot runs a job exactly once at StartUnixNano.
	ScheduleKindOneShot ScheduleKind = 1

	// ScheduleKindFixedInterval runs a job every IntervalMillis starting
	// at StartUnixNano.
	ScheduleKindFixedInterval ScheduleKind = 2
)

// Backfill modes.
const (
	// BackfillModeInherit applies the library-instance default, which in
	// turn defaults to enabled.
	BackfillModeInherit BackfillMode = 0

	// BackfillModeEnabled materializes and dispatches missed occurrences.
	BackfillModeEnabled BackfillMode = 1

	// BackfillModeDisabled skips missed occurrences and resumes at the
	// next future occurrence.
	BackfillModeDisabled BackfillMode = 2
)

// Retry policies.
const (
	// RetryPolicyInherit applies the scheduler default: DefaultMaxRetries
	// retries with exponential delay.
	RetryPolicyInherit RetryPolicy = 0

	// RetryPolicyNone disables function-error retries.
	RetryPolicyNone RetryPolicy = 1

	// RetryPolicyImmediate retries with no delay.
	RetryPolicyImmediate RetryPolicy = 2

	// RetryPolicyFixed retries after InitialDelayMillis every time.
	RetryPolicyFixed RetryPolicy = 3

	// RetryPolicyExponential retries after an exponentially growing
	// delay, capped at MaxDelayMillis.
	RetryPolicyExponential RetryPolicy = 4
)

// Overlap policies.
const (
	// OverlapPolicyInherit applies the scheduler default, which is
	// OverlapPolicyAllow.
	OverlapPolicyInherit OverlapPolicy = 0

	// OverlapPolicyAllow lets occurrences of one job run concurrently.
	OverlapPolicyAllow OverlapPolicy = 1

	// OverlapPolicySkip skips an occurrence that becomes due while a
	// previous occurrence of the same job still runs.
	OverlapPolicySkip OverlapPolicy = 2
)

// DefaultMaxRetries is the default number of function-error retries after
// the initial execution.
const DefaultMaxRetries uint32 = 3

const (
	boolFalse uint8 = 0
	boolTrue  uint8 = 1
)

// payloadReadChunk bounds how much frame payload ReadFrame reserves ahead
// of the bytes that have arrived, so a declared length cannot be turned
// into an allocation by a peer that never sends the payload.
const payloadReadChunk uint32 = 8 << 10

// minFunctionSpecSize is the smallest wire size of one function spec: four
// empty length-prefixed strings. A declared spec count above the remaining
// payload divided by this cannot be satisfied, so it is rejected before
// any spec is allocated.
const minFunctionSpecSize = 16

const logTag = "[SCHEDULERPROTOCOL]"
