package engine

import (
	"context"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

func (e *engine) HandleResultAck(
	ctx context.Context,
	instanceID protocol.InstanceID,
	ack *protocol.ResultDeliveryAck,
) {
	if !e.resultOwnedBy(ctx, ack.JobUUID, instanceID) {
		e.metrics.StaleMessages.Add(1)
		e.log.Warnf(
			logTag+" result ack for job %s refused: instance %s holds no such result",
			ack.JobUUID,
			instanceID,
		)

		return
	}

	if !ack.Accepted {
		abandoned, abandonErr := e.results.DeleteResult(ctx, ack.JobUUID)
		if abandonErr != nil {
			e.log.Errorf(logTag+" result deletion failed: %v", abandonErr)

			return
		}

		if !abandoned {
			e.metrics.StaleMessages.Add(1)

			return
		}

		e.metrics.ResultsAbandoned.Add(1)
		e.log.Warnf(logTag+" result of job %s refused by its submitter", ack.JobUUID)

		return
	}

	deleted, err := e.results.DeleteResult(ctx, ack.JobUUID)
	if err != nil {
		e.log.Errorf(logTag+" result deletion failed: %v", err)

		return
	}

	if !deleted {
		e.metrics.StaleMessages.Add(1)

		return
	}

	e.metrics.ResultsAcked.Add(1)
	e.log.Debugf(logTag+" result of job %s acknowledged and released", ack.JobUUID)
}

func (e *engine) HandleRegistered(
	ctx context.Context,
	instanceID protocol.InstanceID,
) {
	pending, err := e.results.ResultsForInstance(ctx, instanceID, 0)
	if err != nil {
		e.log.Errorf(logTag+" pending result lookup failed: %v", err)

		return
	}

	for _, result := range pending {
		e.deliverResult(ctx, result)
	}
}

// captureResult holds the settled outcome of one execution for the
// submitter that asked for it. It runs only after the terminal state
// transition succeeded, and a refused store never fails the execution:
// the result is dropped, counted, and logged instead.
func (e *engine) captureResult(
	ctx context.Context,
	job *store.Job,
	execution *store.Execution,
	result *protocol.ExecResult,
) {
	if job.ResultMode != protocol.ResultModeDeliver {
		return
	}

	pending := &store.PendingResult{
		JobUUID:     job.ID,
		InstanceID:  job.SubmitterInstanceID,
		ExecutionID: execution.ID,
		Success:     store.Delivered(result.Success),
		HasValue:    store.HasValue(result.HasValue),
		Payload:     store.Payload(result.Result),
		Cause:       result.Error,
	}

	if !e.resultWithinCaps(ctx, job.SubmitterInstanceID) {
		e.metrics.ResultsDropped.Add(1)
		e.log.Warnf(logTag+" result of job %s dropped: pending-result cap reached", job.ID)

		return
	}

	stored, err := e.results.StoreResult(ctx, pending)
	if err != nil {
		e.log.Errorf(logTag+" result store failed: %v", err)

		return
	}

	if !stored {
		e.metrics.ResultsDropped.Add(1)
		e.log.Warnf(logTag+" result of job %s dropped: storage refused it", job.ID)

		return
	}

	e.metrics.ResultsStored.Add(1)
	e.deliverResult(ctx, pending)
}

// resultWithinCaps reports whether one more pending result fits both the
// global and the per-instance budget. The per-instance cap is the
// load-bearing one: a global cap alone would let one flapping submitter
// starve every other caller.
func (e *engine) resultWithinCaps(
	ctx context.Context,
	instanceID protocol.InstanceID,
) (allowed bool) {
	total, err := e.results.CountResults(ctx)
	if err != nil {
		e.log.Errorf(logTag+" result count lookup failed: %v", err)

		return true
	}

	if total >= e.cfg.MaxPendingResults {
		return false
	}

	held, heldErr := e.results.ResultsForInstance(ctx, instanceID, 0)
	if heldErr != nil {
		e.log.Errorf(logTag+" pending result lookup failed: %v", heldErr)

		return true
	}

	return store.OccurrenceCount(len(held)) < e.cfg.MaxPendingResultsPerInstance
}

// deliverResult sends one held result towards its submitter when that
// submitter is currently registered and alive. The held entry survives the
// send: enqueued is not received, so only an acknowledgement may delete it.
func (e *engine) deliverResult(
	ctx context.Context,
	result *store.PendingResult,
) (sent bool) {
	entry, found := e.registry.Get(result.InstanceID)
	if !found || !entry.Alive() {
		return false
	}

	delivery := &protocol.ResultDelivery{
		JobUUID:     result.JobUUID,
		ExecutionID: result.ExecutionID,
		Success:     bool(result.Success),
		HasValue:    bool(result.HasValue),
		Result:      []byte(result.Payload),
		Error:       result.Cause,
	}

	if err := entry.Enqueue(delivery); err != nil {
		e.log.Warnf(logTag+" result delivery enqueue failed: %v", err)

		return false
	}

	if err := e.results.MarkResultSent(ctx, result.JobUUID, e.now()); err != nil {
		e.log.Warnf(logTag+" result send recording failed: %v", err)
	}

	if result.Attempts == 0 {
		e.metrics.ResultsDelivered.Add(1)
	} else {
		e.metrics.ResultsRedelivered.Add(1)
	}

	e.log.WithFields(map[string]any{
		"job_id":           result.JobUUID.String(),
		logFieldInstanceID: string(result.InstanceID),
	}).Info(logTag + " result delivered to submitter")

	return true
}

// sweepResults is the reconcile pass over held results: expiry first, so a
// result about to be evicted is not redelivered in the same pass, then a
// bounded redelivery round for connected submitters that never answered.
func (e *engine) sweepResults(ctx context.Context, now time.Time) {
	e.expireResults(ctx, now)
	e.redeliverStaleResults(ctx, now)
}

// expireResults evicts held results older than the retention budget.
// Expiry keys on CreatedAt: retention bounds total hold time, so a result
// endlessly redelivered to a flapping caller cannot postpone its own
// eviction by being sent again.
func (e *engine) expireResults(ctx context.Context, now time.Time) {
	expired, err := e.results.ExpiredResults(
		ctx,
		now.Add(-e.cfg.ResultRetention),
		e.cfg.DispatchBatch,
	)
	if err != nil {
		e.log.Errorf(logTag+" expired result lookup failed: %v", err)

		return
	}

	for _, result := range expired {
		deleted, deleteErr := e.results.DeleteResult(ctx, result.JobUUID)
		if deleteErr != nil {
			e.log.Errorf(logTag+" expired result deletion failed: %v", deleteErr)

			continue
		}

		if deleted {
			e.metrics.ResultsExpired.Add(1)
			e.log.Warnf(
				logTag+" result of job %s evicted: retention ran out unacknowledged",
				result.JobUUID,
			)
		}
	}
}

func (e *engine) redeliverStaleResults(ctx context.Context, now time.Time) {
	budget := e.cfg.DispatchBatch
	cutoff := now.Add(-e.cfg.RedispatchDelay)

	for _, instanceID := range e.registry.ConnectedInstances() {
		if budget <= 0 {
			return
		}

		pending, err := e.results.ResultsForInstance(ctx, instanceID, budget)
		if err != nil {
			e.log.Errorf(logTag+" pending result lookup failed: %v", err)

			continue
		}

		for _, result := range pending {
			if budget <= 0 {
				return
			}

			if result.LastSentAt.After(cutoff) {
				continue
			}

			if e.deliverResult(ctx, result) {
				budget--
			}
		}
	}
}

// resultOwnedBy reports whether the pending result of one job is held for
// the given instance. Ownership gates the ack path: a forged ack from
// another executor must never delete someone else's result.
func (e *engine) resultOwnedBy(
	ctx context.Context,
	jobUUID protocol.JobUUID,
	instanceID protocol.InstanceID,
) (owned bool) {
	held, err := e.results.ResultsForInstance(ctx, instanceID, 0)
	if err != nil {
		e.log.Errorf(logTag+" pending result lookup failed: %v", err)

		return false
	}

	for _, result := range held {
		if result.JobUUID == jobUUID {
			return true
		}
	}

	return false
}
