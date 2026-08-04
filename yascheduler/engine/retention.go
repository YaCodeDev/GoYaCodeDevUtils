package engine

import (
	"context"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// sweepExecutions is the reconcile pass over settled executions: it purges
// every execution whose retention window elapsed, cascading to its
// attempts first so no attempt ever outlives the execution it belongs to.
func (e *engine) sweepExecutions(ctx context.Context, now time.Time) {
	expired, err := e.executions.ExpiredExecutions(
		ctx,
		now.Add(-e.cfg.ExecutionRetention),
		e.cfg.DispatchBatch,
	)
	if err != nil {
		e.log.Errorf(logTag+" expired execution lookup failed: %v", err)

		return
	}

	for _, execution := range expired {
		e.purgeExecution(ctx, execution)
	}
}

// purgeExecution deletes one settled execution and every attempt it owns.
// Attempts are deleted first, and any single failure — listing them or
// deleting one — aborts the whole purge for this execution without
// deleting the execution itself: ExpiredExecutions is keyed off settle
// time, which only moves further into the past, so a failed purge stays in
// the expired set and is retried whole on the next reconcile pass. That
// also guarantees an attempt is never orphaned by a deleted owning
// execution, which nothing else in this codebase would ever clean up.
func (e *engine) purgeExecution(ctx context.Context, execution *store.Execution) {
	attempts, err := e.attempts.AttemptsForExecution(ctx, execution.ID)
	if err != nil {
		e.executionLog(execution).Errorf(logTag+" purge attempt listing failed: %v", err)

		return
	}

	for _, attempt := range attempts {
		deleted, deleteErr := e.attempts.DeleteAttempt(ctx, attempt.ID)
		if deleteErr != nil {
			e.executionLog(execution).
				Errorf(logTag+" purge attempt deletion failed: %v", deleteErr)

			return
		}

		if deleted {
			e.metrics.AttemptsPurged.Add(1)
		}
	}

	deleted, deleteErr := e.executions.DeleteExecution(ctx, execution.ID)
	if deleteErr != nil {
		e.executionLog(execution).Errorf(logTag+" purge execution deletion failed: %v", deleteErr)

		return
	}

	if deleted {
		e.metrics.ExecutionsExpired.Add(1)
		e.executionLog(execution).Debug(logTag + " execution purged: retention ran out")
	}
}
