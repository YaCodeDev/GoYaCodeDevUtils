package store

import "errors"

// Store contract violations. Every implementation raises these sentinels,
// so a caller matches on the condition rather than on the backend.
var (
	// ErrNilJob reports an upsert handed a nil job.
	ErrNilJob = errors.New("job is nil")

	// ErrZeroJobUUID reports an upsert handed the all-zero job identifier,
	// which no minted identifier ever is and which every such job would
	// otherwise collide on.
	ErrZeroJobUUID = errors.New("job uuid is zero")

	// ErrNilResult reports a result store handed a nil pending result.
	ErrNilResult = errors.New("pending result is nil")

	// ErrJobNotFound reports a job identifier or key with no stored job.
	ErrJobNotFound = errors.New("job not found")

	// ErrExecutionNotFound reports an execution identifier with no stored
	// execution.
	ErrExecutionNotFound = errors.New("execution not found")

	// ErrAttemptNotFound reports an attempt identifier with no stored
	// attempt.
	ErrAttemptNotFound = errors.New("attempt not found")

	// ErrResultNotFound reports a job identifier with no pending result.
	ErrResultNotFound = errors.New("pending result not found")

	// ErrVersionConflict reports an update whose stated version no longer
	// matches the stored record.
	ErrVersionConflict = errors.New("execution version conflict")

	// ErrTerminalState reports a state change asked of a settled
	// execution.
	ErrTerminalState = errors.New("execution is in a terminal state")

	// ErrIllegalTransition reports a state change the transition table
	// does not allow.
	ErrIllegalTransition = errors.New("illegal execution state transition")
)
