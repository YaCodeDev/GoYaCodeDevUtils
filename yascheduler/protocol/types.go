package protocol

// MessageType identifies the payload layout carried by a frame.
type MessageType uint8

// ErrorCode classifies a structured wire error.
type ErrorCode uint16

// CorrelationID matches a response frame to the request frame it answers.
// It is minted by the side that sends the request and echoed back verbatim.
type CorrelationID uint64

// JobID is the scheduler-minted identifier of one job definition.
type JobID uint64

// ExecutionID is the scheduler-minted identifier of one scheduled
// occurrence of a job.
type ExecutionID uint64

// AttemptID is the scheduler-minted identifier of one delivery or
// execution attempt of an execution.
type AttemptID uint64

// ExecutorType names the kind of service an executor process is.
type ExecutorType string

// InstanceID identifies one running executor process. It stays stable
// across reconnects of the same process and changes when the process is
// recreated.
type InstanceID string

// FunctionName is the stable registered name of an executable function.
type FunctionName string

// FunctionVersion is the optional version tag of a registered function.
type FunctionVersion string

// ScheduleKind selects the schedule algorithm of a job.
type ScheduleKind uint8

// BackfillMode selects how missed occurrences are handled for a job.
type BackfillMode uint8

// RetryPolicy selects the delay strategy applied between function retries.
type RetryPolicy uint8

// OverlapPolicy selects what happens when an occurrence becomes due while
// a previous occurrence of the same job is still running.
type OverlapPolicy uint8

// WireError is the structured error representation carried by protocol
// messages. Retryable reports whether the sender considers the failed
// operation safe to retry.
type WireError struct {
	Code      ErrorCode
	Retryable bool
	Message   string
}

// FunctionSpec describes one executable function: its stable name, its
// optional version tag, and opaque input and output signature strings.
// Signatures are compared byte-wise for compatibility; they carry no
// wire-level semantics beyond equality.
type FunctionSpec struct {
	Name            FunctionName
	Version         FunctionVersion
	InputSignature  string
	OutputSignature string
}

// ScheduleSpec describes when a job runs. StartUnixNano anchors the
// schedule (and is the run time for one-shot jobs); IntervalMillis is the
// period for fixed-interval jobs and must be zero for one-shot jobs. All
// times are UTC unix nanoseconds.
type ScheduleSpec struct {
	Kind           ScheduleKind
	StartUnixNano  int64
	IntervalMillis uint64
}

// BackfillSpec configures missed-occurrence handling for one job. Mode
// selects enabled, disabled, or inheritance of the instance-level default.
// Zero MaxCount or MaxAgeMillis inherit the scheduler defaults.
type BackfillSpec struct {
	Mode         BackfillMode
	MaxCount     uint32
	MaxAgeMillis uint64
}

// RetrySpec configures function-error retries for one job. Policy selects
// the delay strategy; RetryPolicyInherit keeps the scheduler default of
// DefaultMaxRetries retries with exponential delay. MultiplierBits carries
// the exponential multiplier as IEEE-754 bits so the wire format stays
// integer-only.
type RetrySpec struct {
	Policy             RetryPolicy
	MaxRetries         uint32
	InitialDelayMillis uint64
	MaxDelayMillis     uint64
	MultiplierBits     uint64
}
