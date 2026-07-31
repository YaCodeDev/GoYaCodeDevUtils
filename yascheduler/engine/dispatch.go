package engine

import (
	"context"
	"math"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

func (e *engine) processDue(ctx context.Context) {
	now := e.now()

	due, err := e.executions.DueExecutions(ctx, now, e.cfg.DispatchBatch)
	if err != nil {
		e.log.Errorf(logTag+" due executions lookup failed: %v", err)

		return
	}

	for _, execution := range due {
		if ctx.Err() != nil || e.stopping.Load() {
			return
		}

		e.dispatchExecution(ctx, execution, now)
	}
}

func (e *engine) dispatchExecution(
	ctx context.Context,
	execution *store.Execution,
	now time.Time,
) {
	log := e.executionLog(execution)

	job, err := e.jobs.GetJob(ctx, execution.JobID)
	if err != nil {
		log.Errorf(logTag+" job lookup failed: %v", err)

		return
	}

	if !job.Enabled {
		e.transition(ctx, execution, store.StateCancelled, func(update *store.ExecutionUpdate) {
			reason := store.WaitReason(cancelReasonJobDisabled)
			update.WaitReason = &reason
		})

		return
	}

	if resolveOverlap(job.Overlap) == protocol.OverlapPolicySkip {
		active, activeErr := e.executions.HasActiveExecution(ctx, job.ID, execution.ID)
		if activeErr == nil && active {
			e.transition(ctx, execution, store.StateSkipped, func(update *store.ExecutionUpdate) {
				reason := store.WaitReason(skipReasonOverlap)
				update.WaitReason = &reason
			})
			e.metrics.SkippedOverlaps.Add(1)
			e.materializeNext(ctx, job, execution.ScheduledAt)

			return
		}
	}

	entry, found := e.registry.Select(job.ExecutorType, &job.Function)
	if !found {
		e.parkUndispatchable(ctx, job, execution)

		return
	}

	dispatching := store.StateDispatching
	lease := now.Add(e.cfg.Lease)

	claimed, claimErr := e.executions.UpdateExecution(
		ctx,
		execution.ID,
		execution.Version,
		store.ExecutionUpdate{State: &dispatching, LeaseExpiresAt: &lease},
	)
	if claimErr != nil {
		log.Debugf(logTag+" claim lost: %v", claimErr)

		return
	}

	priorAttempts, attemptsErr := e.attempts.AttemptsForExecution(ctx, execution.ID)
	if attemptsErr != nil {
		log.Errorf(logTag+" attempt history lookup failed: %v", attemptsErr)

		return
	}

	attempt, createErr := e.attempts.CreateAttempt(
		ctx,
		execution.ID,
		nextAttemptNumber(len(priorAttempts)),
		entry.InstanceID(),
	)
	if createErr != nil {
		log.Errorf(logTag+" attempt creation failed: %v", createErr)

		return
	}

	fenced, fenceErr := e.executions.UpdateExecution(
		ctx,
		claimed.ID,
		claimed.Version,
		store.ExecutionUpdate{CurrentAttemptID: &attempt.ID},
	)
	if fenceErr != nil {
		log.Errorf(logTag+" attempt fencing failed: %v", fenceErr)

		return
	}

	request := &protocol.ExecRequest{
		JobUUID:           job.ID,
		ExecutionID:       execution.ID,
		AttemptID:         attempt.ID,
		AttemptNumber:     uint32(fenced.FunctionAttempts) + firstAttemptNumber,
		Function:          job.Function,
		Args:              []byte(job.Args),
		ScheduledUnixNano: execution.ScheduledAt.UnixNano(),
		TimeoutMillis:     executionTimeoutMillisNone,
	}

	entry.AddInFlight(attempt.ID)

	if enqueueErr := entry.Enqueue(request); enqueueErr != nil {
		entry.ReleaseInFlight(attempt.ID)

		if _, updateErr := e.attempts.UpdateAttemptState(
			ctx,
			attempt.ID,
			[]store.AttemptState{store.AttemptDispatched},
			store.AttemptInfraFailed,
			dispatchReasonQueueFull,
		); updateErr != nil {
			log.Errorf(logTag+" attempt update failed: %v", updateErr)
		}

		e.requeueReady(ctx, execution.ID, e.cfg.RedispatchDelay)
		e.metrics.DispatchFailures.Add(1)

		log.Warnf(logTag+" dispatch enqueue failed: %v", enqueueErr)

		return
	}

	e.metrics.Dispatches.Add(1)

	log.WithFields(map[string]any{
		"attempt_id":  uint64(attempt.ID),
		"instance_id": string(entry.InstanceID()),
	}).Info(logTag + " execution dispatched")

	e.materializeNext(ctx, job, execution.ScheduledAt)
}

func (e *engine) parkUndispatchable(
	ctx context.Context,
	job *store.Job,
	execution *store.Execution,
) {
	if e.registry.PoolSize(job.ExecutorType) == 0 {
		e.transition(
			ctx,
			execution,
			store.StateWaitingExecutor,
			func(update *store.ExecutionUpdate) {
				reason := store.WaitReason(waitReasonNoExecutor)
				update.WaitReason = &reason
			},
		)
		e.metrics.WaitingExecutor.Add(1)

		return
	}

	if e.registry.SupportsFunction(job.ExecutorType, &job.Function) {
		e.requeueReady(ctx, execution.ID, e.cfg.RedispatchDelay)
		e.metrics.WaitingCapacity.Add(1)

		return
	}

	e.transition(
		ctx,
		execution,
		store.StateWaitingCompatible,
		func(update *store.ExecutionUpdate) {
			reason := store.WaitReason(waitReasonNoCompatible)
			update.WaitReason = &reason
		},
	)
	e.metrics.WaitingCompatible.Add(1)
}

func (e *engine) transition(
	ctx context.Context,
	execution *store.Execution,
	state store.ExecutionState,
	mutate func(update *store.ExecutionUpdate),
) {
	update := store.ExecutionUpdate{State: &state}

	if mutate != nil {
		mutate(&update)
	}

	if _, err := e.executions.UpdateExecution(
		ctx,
		execution.ID,
		execution.Version,
		update,
	); err != nil {
		e.executionLog(execution).Debugf(
			logTag+" transition to %s lost: %v",
			state.String(),
			err,
		)
	}
}

func (e *engine) requeueReady(
	ctx context.Context,
	executionID protocol.ExecutionID,
	delay time.Duration,
) (requeued bool) {
	execution, err := e.executions.GetExecution(ctx, executionID)
	if err != nil {
		return false
	}

	if execution.State.Terminal() {
		return false
	}

	ready := store.StateReady
	next := e.now().Add(delay)

	if _, err = e.executions.UpdateExecution(
		ctx,
		execution.ID,
		execution.Version,
		store.ExecutionUpdate{State: &ready, NextAttemptAt: &next},
	); err != nil {
		return false
	}

	return true
}

func (e *engine) abandonAttempt(
	ctx context.Context,
	execution *store.Execution,
	reason store.ErrorText,
) {
	if execution.CurrentAttemptID != 0 {
		if _, err := e.attempts.UpdateAttemptState(
			ctx,
			execution.CurrentAttemptID,
			[]store.AttemptState{store.AttemptDispatched, store.AttemptAccepted},
			store.AttemptLost,
			reason,
		); err != nil {
			e.executionLog(execution).Errorf(logTag+" attempt abandon failed: %v", err)
		}

		e.releaseAttemptSlot(ctx, execution.CurrentAttemptID)
	}

	if e.requeueReady(ctx, execution.ID, 0) {
		e.metrics.InfraRedispatches.Add(1)
	}
}

func (e *engine) releaseAttemptSlot(
	ctx context.Context,
	attemptID protocol.AttemptID,
) {
	attempt, err := e.attempts.GetAttempt(ctx, attemptID)
	if err != nil {
		return
	}

	if entry, found := e.registry.Get(attempt.InstanceID); found {
		entry.ReleaseInFlight(attemptID)
	}
}

func (e *engine) materializeNext(
	ctx context.Context,
	job *store.Job,
	afterOccurrence time.Time,
) {
	occurrence, exists := nextOccurrence(job.Schedule, afterOccurrence)
	if !exists {
		return
	}

	_, created, err := e.executions.CreateExecution(
		ctx,
		job.ID,
		occurrence,
		store.StateScheduled,
		false,
	)
	if err != nil {
		e.log.Errorf(logTag+" next occurrence creation failed: %v", err)

		return
	}

	if created {
		e.Notify()
	}
}

func (e *engine) executionLog(execution *store.Execution) (log yalogger.Logger) {
	return e.log.WithFields(map[string]any{
		"job_id":       execution.JobID.String(),
		"execution_id": uint64(execution.ID),
		"scheduled_at": execution.ScheduledAt.Format(time.RFC3339Nano),
		"state":        execution.State.String(),
	})
}

// nextAttemptNumber turns a prior-attempt count into the one-based ordinal
// of the attempt about to be created, saturating rather than wrapping an
// implausibly long history back to zero.
func nextAttemptNumber(priorAttempts int) (number store.AttemptNumber) {
	if priorAttempts < 0 || priorAttempts >= math.MaxUint32 {
		return math.MaxUint32
	}

	return store.AttemptNumber(priorAttempts) + firstAttemptNumber
}

func resolveOverlap(policy protocol.OverlapPolicy) (resolved protocol.OverlapPolicy) {
	if policy == protocol.OverlapPolicyInherit {
		return protocol.OverlapPolicyAllow
	}

	return policy
}
