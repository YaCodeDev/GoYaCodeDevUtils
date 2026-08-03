package protocol

const (
	// Magic is the 4-byte preamble ("YASC") every yascheduler frame starts with.
	Magic uint32 = 0x59415343

	// Version1 is the first protocol version. It is no longer spoken: it
	// is kept named so a rejected version byte can be recognised.
	Version1 uint8 = 1

	// Version2 widens job identifiers to 128 bits and adds label routing,
	// result delivery, and job pinning. It is no longer spoken: it is
	// kept named so a rejected version byte can be recognised.
	Version2 uint8 = 2

	// Version3 adds job deletion: the JobDelete and JobDeleteAck message
	// types.
	Version3 uint8 = 3

	// CurrentVersion is the protocol version this package speaks.
	CurrentVersion = Version3

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

	// DefaultMaxLabelLen caps one routing label at 128 bytes.
	DefaultMaxLabelLen uint32 = 1 << 7

	// DefaultMaxLabels caps one label set at 64 labels.
	DefaultMaxLabels uint32 = 1 << 6

	// DefaultMaxResultBytes caps one delivered result at 64 KiB. It is
	// deliberately separate from and smaller than DefaultMaxBytesLen
	// because a result is held in memory across a caller disconnect, so
	// this cap multiplies by the pending-result cap instead of bounding a
	// single in-flight message.
	DefaultMaxResultBytes uint32 = 1 << 16
)

// Message types of protocol version 3.
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

	// MessageTypeLabelUpdate announces and withdraws routing labels on a
	// live executor connection.
	MessageTypeLabelUpdate MessageType = 13

	// MessageTypeLabelUpdateAck answers a label update.
	MessageTypeLabelUpdateAck MessageType = 14

	// MessageTypeResultDelivery hands an execution result back to the
	// executor connection that requested it.
	MessageTypeResultDelivery MessageType = 15

	// MessageTypeResultDeliveryAck answers a result delivery.
	MessageTypeResultDeliveryAck MessageType = 16

	// MessageTypeJobDelete withdraws a stored job definition.
	MessageTypeJobDelete MessageType = 17

	// MessageTypeJobDeleteAck answers a job delete.
	MessageTypeJobDeleteAck MessageType = 18
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

	// ErrorCodeLabelRejected reports a label announcement the scheduler
	// refused.
	ErrorCodeLabelRejected ErrorCode = 15

	// ErrorCodeResultNotRequested reports a result delivered for a job
	// whose result mode is ResultModeIgnore.
	ErrorCodeResultNotRequested ErrorCode = 16

	// ErrorCodeNoLabeledExecutor reports a job whose pin label matches no
	// connected executor under PinPolicyStrict.
	ErrorCodeNoLabeledExecutor ErrorCode = 17
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

// Pin policies.
const (
	// PinPolicyStrict runs a pinned job only on an executor announcing
	// the pin label. It is the zero value, so an unset policy never
	// widens routing.
	PinPolicyStrict PinPolicy = 0

	// PinPolicyPreferred prefers an executor announcing the pin label and
	// falls back to any executor of the job's type when none is
	// connected.
	PinPolicyPreferred PinPolicy = 1
)

// Result modes.
const (
	// ResultModeIgnore discards the execution result once the attempt
	// settles. It is the zero value.
	ResultModeIgnore ResultMode = 0

	// ResultModeDeliver holds the execution result and delivers it to the
	// connection that requested the job. A repeating job holds at most
	// one result: each settled occurrence replaces a still-held one.
	ResultModeDeliver ResultMode = 1
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

// minLabelSize is the smallest wire size of one label: an empty
// length-prefixed string. A declared label count above the remaining
// payload divided by this cannot be satisfied, so it is rejected before
// any label is allocated.
const minLabelSize = 4

// uuidSize is the wire width of a JobUUID.
const uuidSize = 16

// uuidStringSize is the length of the canonical 8-4-4-4-12 rendering of a
// JobUUID.
const uuidStringSize = 36

// uuidStringDash separates the groups of the canonical rendering.
const uuidStringDash = '-'

const logTag = "[SCHEDULERPROTOCOL]"
