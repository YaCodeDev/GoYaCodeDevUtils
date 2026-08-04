package store

import (
	"context"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// JobRepository persists job definitions. Job keys are scoped by executor
// type: the same key under two executor types addresses two distinct jobs.
// DeleteJob removes the stored job and frees its executor-scoped key,
// reporting false with no error when no job was stored, so a replayed
// delete is idempotent; executions and pending results of the job are the
// engine's to clean.
type JobRepository interface {
	UpsertJob(ctx context.Context, job *Job) (*Job, yaerrors.Error)
	GetJob(ctx context.Context, id protocol.JobUUID) (*Job, yaerrors.Error)
	GetJobByKey(
		ctx context.Context,
		executorType protocol.ExecutorType,
		key JobKey,
	) (*Job, yaerrors.Error)
	DeleteJob(ctx context.Context, id protocol.JobUUID) (bool, yaerrors.Error)
	SetJobEnabled(
		ctx context.Context,
		id protocol.JobUUID,
		enabled Enabled,
	) yaerrors.Error
	AddSkippedOccurrences(
		ctx context.Context,
		id protocol.JobUUID,
		count OccurrenceCount,
	) yaerrors.Error
	ListEnabledJobs(ctx context.Context) ([]*Job, yaerrors.Error)
}

// ExecutionRepository persists materialized occurrences of jobs.
// DeleteExecution removes the stored execution, reporting false with no
// error when none was stored, so a replayed delete is idempotent; cleaning
// up the execution's attempts is the caller's job, not the store's.
// ExpiredExecutions returns terminal executions that settled before a
// cutoff, mirroring ExpiredResults' retention contract.
type ExecutionRepository interface {
	CreateExecution(
		ctx context.Context,
		jobID protocol.JobUUID,
		scheduledAt time.Time,
		state ExecutionState,
		backfilled Backfilled,
	) (*Execution, bool, yaerrors.Error)
	GetExecution(
		ctx context.Context,
		id protocol.ExecutionID,
	) (*Execution, yaerrors.Error)
	UpdateExecution(
		ctx context.Context,
		id protocol.ExecutionID,
		expectedVersion Version,
		update ExecutionUpdate,
	) (*Execution, yaerrors.Error)
	DueExecutions(
		ctx context.Context,
		now time.Time,
		limit BatchLimit,
	) ([]*Execution, yaerrors.Error)
	NextWakeAt(ctx context.Context) (time.Time, bool, yaerrors.Error)
	ExecutionsInStates(
		ctx context.Context,
		states ...ExecutionState,
	) ([]*Execution, yaerrors.Error)
	ExecutionsForJob(
		ctx context.Context,
		jobID protocol.JobUUID,
	) ([]*Execution, yaerrors.Error)
	HasActiveExecution(
		ctx context.Context,
		jobID protocol.JobUUID,
		exclude protocol.ExecutionID,
	) (bool, yaerrors.Error)
	HasPendingOccurrence(
		ctx context.Context,
		jobID protocol.JobUUID,
	) (bool, yaerrors.Error)
	ExpiredLeases(
		ctx context.Context,
		now time.Time,
	) ([]*Execution, yaerrors.Error)
	DeleteExecution(ctx context.Context, id protocol.ExecutionID) (bool, yaerrors.Error)
	ExpiredExecutions(
		ctx context.Context,
		before time.Time,
		limit BatchLimit,
	) ([]*Execution, yaerrors.Error)
}

// AttemptRepository persists deliveries of executions to executor
// instances. DeleteAttempt removes the stored attempt, reporting false
// with no error when none was stored, so a replayed delete is idempotent.
type AttemptRepository interface {
	CreateAttempt(
		ctx context.Context,
		executionID protocol.ExecutionID,
		number AttemptNumber,
		instanceID protocol.InstanceID,
	) (*Attempt, yaerrors.Error)
	GetAttempt(
		ctx context.Context,
		id protocol.AttemptID,
	) (*Attempt, yaerrors.Error)
	UpdateAttemptState(
		ctx context.Context,
		id protocol.AttemptID,
		from []AttemptState,
		to AttemptState,
		errorText ErrorText,
	) (bool, yaerrors.Error)
	AttemptsForExecution(
		ctx context.Context,
		executionID protocol.ExecutionID,
	) ([]*Attempt, yaerrors.Error)
	AttemptsOnInstance(
		ctx context.Context,
		instanceID protocol.InstanceID,
		states ...AttemptState,
	) ([]*Attempt, yaerrors.Error)
	DeleteAttempt(ctx context.Context, id protocol.AttemptID) (bool, yaerrors.Error)
}

// ResultRepository persists settled execution results awaiting delivery to
// the submitter that asked for them. StoreResult reports false when a
// storage cap refuses the result rather than failing the execution.
type ResultRepository interface {
	StoreResult(ctx context.Context, result *PendingResult) (bool, yaerrors.Error)
	DeleteResult(ctx context.Context, jobUUID protocol.JobUUID) (bool, yaerrors.Error)
	ResultsForInstance(
		ctx context.Context,
		id protocol.InstanceID,
		limit BatchLimit,
	) ([]*PendingResult, yaerrors.Error)
	MarkResultSent(
		ctx context.Context,
		jobUUID protocol.JobUUID,
		at time.Time,
	) yaerrors.Error
	ExpiredResults(
		ctx context.Context,
		before time.Time,
		limit BatchLimit,
	) ([]*PendingResult, yaerrors.Error)
	CountResults(ctx context.Context) (OccurrenceCount, yaerrors.Error)
}

// Store is the whole persistence surface a scheduler engine needs. The
// granular repositories stay separate so a constructor can take only the
// part it uses.
type Store interface {
	JobRepository
	ExecutionRepository
	AttemptRepository
	ResultRepository
}
