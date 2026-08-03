package store

import "fmt"

type (
	// JobKey is the caller-chosen stable name of a job definition, scoped
	// by the job's executor type. Two upserts carrying the same executor
	// type and key address the same job.
	JobKey string

	// ErrorText is a human-readable failure description recorded on an
	// execution or an attempt.
	ErrorText string

	// WaitReason explains why an execution is parked in a waiting state.
	WaitReason string

	// Reason explains why an operation was refused or an occurrence was
	// skipped.
	Reason string

	// Version is the optimistic-concurrency counter of a stored record. An
	// update states the version it read and fails when the record moved on.
	Version uint64

	// AttemptNumber is the one-based ordinal of an attempt within its
	// execution.
	AttemptNumber uint32

	// FunctionAttempts counts how many times the function of one execution
	// has been invoked.
	FunctionAttempts uint32

	// OccurrenceCount counts scheduled occurrences of a job.
	OccurrenceCount uint64

	// Payload is an opaque argument or result byte string. It redacts
	// itself in logs.
	Payload []byte

	// Enabled reports whether a job is eligible for scheduling.
	Enabled bool

	// Backfilled reports whether an execution materializes a missed
	// occurrence rather than a live one.
	Backfilled bool

	// BatchLimit caps how many records one query returns. A non-positive
	// limit means unlimited.
	BatchLimit int

	// PoolSize is the number of workers in a pool.
	PoolSize int

	// Generation counts restarts of a component whose in-flight work must
	// be told apart across a restart.
	Generation uint64

	// Capacity is the number of concurrent executions a party accepts.
	Capacity uint32

	// InFlight counts operations currently outstanding.
	InFlight int64

	// UnixNano is a UTC instant in nanoseconds since the unix epoch.
	UnixNano int64

	// LabelCount counts routing labels announced by an executor or held in
	// a label set.
	LabelCount uint32

	// ResultAttempts counts how many times a pending result has been sent
	// towards its submitter.
	ResultAttempts uint32

	// Delivered reports whether an execution settled successfully.
	Delivered bool

	// HasValue reports whether a pending result carries a return payload.
	HasValue bool
)

// LogString renders the payload as its byte length, so argument and result
// bytes never reach a log.
func (p Payload) LogString() (redacted string) {
	return fmt.Sprintf("[PAYLOAD %d bytes]", len(p))
}
