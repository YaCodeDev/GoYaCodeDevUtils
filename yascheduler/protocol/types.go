package protocol

import "encoding/hex"

// MessageType identifies the payload layout carried by a frame.
type MessageType uint8

// ErrorCode classifies a structured wire error.
type ErrorCode uint16

// CorrelationID matches a response frame to the request frame it answers.
// It is minted by the side that sends the request and echoed back verbatim.
type CorrelationID uint64

// JobUUID is the 128-bit identifier of one job definition. It is minted by
// the client that upserts the job and travels verbatim on the wire, so the
// same job keeps one identity across schedulers, restarts, and local mode.
type JobUUID [16]byte

// String renders the canonical 8-4-4-4-12 hexadecimal form.
func (j JobUUID) String() string {
	var buf [uuidStringSize]byte

	hex.Encode(buf[0:8], j[0:4])
	buf[8] = uuidStringDash
	hex.Encode(buf[9:13], j[4:6])
	buf[13] = uuidStringDash
	hex.Encode(buf[14:18], j[6:8])
	buf[18] = uuidStringDash
	hex.Encode(buf[19:23], j[8:10])
	buf[23] = uuidStringDash
	hex.Encode(buf[24:36], j[10:16])

	return string(buf[:])
}

// IsZero reports whether the identifier is the all-zero value, which no
// minted job identifier ever is.
func (j JobUUID) IsZero() bool {
	return j == JobUUID{}
}

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

// Label is one routing label an executor announces and a job pins to.
// Labels are compared byte-wise; they carry no structure at this layer.
type Label string

// PinPolicy selects how strictly a job's pin label constrains routing.
type PinPolicy uint8

// ResultMode selects whether the scheduler delivers an execution result
// back to the caller that requested the job.
type ResultMode uint8

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

// PinSpec constrains which executors may run a job. An empty Label pins
// nothing and lets the job run on any executor of its type. Policy is
// PinPolicyStrict in the zero value, so an unset policy never widens
// routing beyond what the label names.
type PinSpec struct {
	Label  Label
	Policy PinPolicy
}
