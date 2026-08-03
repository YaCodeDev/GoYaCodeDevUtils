package store

import (
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

type (
	// ExecutionState is the lifecycle position of one scheduled
	// occurrence.
	ExecutionState uint8

	// AttemptState is the lifecycle position of one delivery of an
	// execution to one executor instance.
	AttemptState uint8
)

// Job is a stored job definition. ID is minted by the client that upserts
// the job, so the same job keeps one identity across schedulers, restarts,
// and local mode; Key is the caller-chosen name that, together with
// ExecutorType, decides which stored job an upsert addresses.
type Job struct {
	ID                  protocol.JobUUID
	Key                 JobKey
	ExecutorType        protocol.ExecutorType
	Function            protocol.FunctionSpec
	Args                Payload
	Schedule            protocol.ScheduleSpec
	Enabled             Enabled
	Backfill            protocol.BackfillSpec
	Retry               protocol.RetrySpec
	Overlap             protocol.OverlapPolicy
	Pin                 protocol.PinSpec
	ResultMode          protocol.ResultMode
	SubmitterInstanceID protocol.InstanceID
	SkippedOccurrences  OccurrenceCount
	Version             Version
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Execution is one materialized occurrence of a job. Version carries the
// optimistic-concurrency counter every update states and the store checks.
type Execution struct {
	ID               protocol.ExecutionID
	JobID            protocol.JobUUID
	ScheduledAt      time.Time
	State            ExecutionState
	FunctionAttempts FunctionAttempts
	CurrentAttemptID protocol.AttemptID
	NextAttemptAt    time.Time
	LeaseExpiresAt   time.Time
	Backfilled       Backfilled
	LastError        ErrorText
	WaitReason       WaitReason
	Version          Version
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Attempt is one delivery of an execution to one executor instance.
type Attempt struct {
	ID          protocol.AttemptID
	ExecutionID protocol.ExecutionID
	Number      AttemptNumber
	InstanceID  protocol.InstanceID
	State       AttemptState
	Error       ErrorText
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ExecutionUpdate is a partial update of one execution. A nil field leaves
// the stored value untouched, so two updates of different fields never
// clobber one another.
type ExecutionUpdate struct {
	State            *ExecutionState
	FunctionAttempts *FunctionAttempts
	CurrentAttemptID *protocol.AttemptID
	NextAttemptAt    *time.Time
	LeaseExpiresAt   *time.Time
	LastError        *ErrorText
	WaitReason       *WaitReason
}

// PendingResult is one settled execution result held for a submitter that
// asked for delivery. It survives a submitter disconnect, so the result of
// a job whose caller dropped is still delivered when that caller returns.
type PendingResult struct {
	JobUUID     protocol.JobUUID
	InstanceID  protocol.InstanceID
	ExecutionID protocol.ExecutionID
	Success     Delivered
	HasValue    HasValue
	Payload     Payload
	Cause       *protocol.WireError
	Attempts    ResultAttempts
	CreatedAt   time.Time
	LastSentAt  time.Time
}

// Terminal reports whether the state is settled, so no further transition
// out of it is legal.
func (s ExecutionState) Terminal() (terminal bool) {
	switch s {
	case StateSucceeded, StateFailed, StateCancelled, StateSkipped:
		return true
	case StateScheduled,
		StateReady,
		StateWaitingExecutor,
		StateWaitingCompatible,
		StateWaitingLabel,
		StateDispatching,
		StateRunning,
		StateRetryWait:
		return false
	default:
		return false
	}
}

// String renders the stable snake_case name of the state, or "unknown" for
// a value outside the defined set.
func (s ExecutionState) String() (name string) {
	switch s {
	case StateScheduled:
		return stateNameScheduled
	case StateReady:
		return stateNameReady
	case StateWaitingExecutor:
		return stateNameWaitingExecutor
	case StateWaitingCompatible:
		return stateNameWaitingCompatible
	case StateWaitingLabel:
		return stateNameWaitingLabel
	case StateDispatching:
		return stateNameDispatching
	case StateRunning:
		return stateNameRunning
	case StateRetryWait:
		return stateNameRetryWait
	case StateSucceeded:
		return stateNameSucceeded
	case StateFailed:
		return stateNameFailed
	case StateCancelled:
		return stateNameCancelled
	case StateSkipped:
		return stateNameSkipped
	default:
		return stateNameUnknown
	}
}
