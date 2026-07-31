package store

// Execution states. An execution walks these states under the transition
// table in transitions.go; the last four are terminal.
const (
	// StateScheduled is a materialized occurrence waiting for its
	// scheduled time.
	StateScheduled ExecutionState = 1

	// StateReady is an occurrence whose time has come and which is waiting
	// for a dispatch slot.
	StateReady ExecutionState = 2

	// StateWaitingExecutor is an occurrence held back because no executor
	// of the job's type is connected.
	StateWaitingExecutor ExecutionState = 3

	// StateWaitingCompatible is an occurrence held back because no
	// connected executor registers a compatible function.
	StateWaitingCompatible ExecutionState = 4

	// StateDispatching is an occurrence handed to an executor and awaiting
	// its acceptance.
	StateDispatching ExecutionState = 5

	// StateRunning is an occurrence an executor accepted and is running.
	StateRunning ExecutionState = 6

	// StateRetryWait is an occurrence whose function failed and which is
	// waiting out its retry delay.
	StateRetryWait ExecutionState = 7

	// StateSucceeded is an occurrence whose function returned without an
	// error.
	StateSucceeded ExecutionState = 8

	// StateFailed is an occurrence that exhausted its retries.
	StateFailed ExecutionState = 9

	// StateCancelled is an occurrence cancelled before it settled.
	StateCancelled ExecutionState = 10

	// StateSkipped is an occurrence never dispatched, because backfill or
	// the overlap policy dropped it.
	StateSkipped ExecutionState = 11

	// StateWaitingLabel is an occurrence held back because no connected
	// executor announces the label its job pins to.
	StateWaitingLabel ExecutionState = 12
)

// Attempt states. One attempt is one delivery of an execution to one
// executor instance.
const (
	// AttemptDispatched is an attempt sent to an executor and awaiting its
	// acceptance.
	AttemptDispatched AttemptState = 1

	// AttemptAccepted is an attempt the executor took responsibility for.
	AttemptAccepted AttemptState = 2

	// AttemptSucceeded is an attempt whose function returned without an
	// error.
	AttemptSucceeded AttemptState = 3

	// AttemptFunctionFailed is an attempt whose function ran and returned
	// an error.
	AttemptFunctionFailed AttemptState = 4

	// AttemptInfraFailed is an attempt that never ran because delivery or
	// the executor itself failed.
	AttemptInfraFailed AttemptState = 5

	// AttemptLost is an attempt whose executor stopped reporting before
	// the attempt settled.
	AttemptLost AttemptState = 6

	// AttemptCancelled is an attempt cancelled before it settled.
	AttemptCancelled AttemptState = 7
)

const (
	stateNameScheduled         = "scheduled"
	stateNameReady             = "ready"
	stateNameWaitingExecutor   = "waiting_executor"
	stateNameWaitingCompatible = "waiting_compatible"
	stateNameWaitingLabel      = "waiting_label"
	stateNameDispatching       = "dispatching"
	stateNameRunning           = "running"
	stateNameRetryWait         = "retry_wait"
	stateNameSucceeded         = "succeeded"
	stateNameFailed            = "failed"
	stateNameCancelled         = "cancelled"
	stateNameSkipped           = "skipped"
	stateNameUnknown           = "unknown"
)
