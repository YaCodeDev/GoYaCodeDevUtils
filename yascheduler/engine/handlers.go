package engine

import (
	"context"
	"slices"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

func (e *engine) HandleJobUpsert(
	ctx context.Context,
	instanceID protocol.InstanceID,
	upsert *protocol.JobUpsert,
) *protocol.JobUpsertAck {
	if reason, valid := validateUpsert(upsert); !valid {
		return &protocol.JobUpsertAck{
			JobKey:   upsert.JobKey,
			JobUUID:  upsert.JobUUID,
			Accepted: false,
			Error: &protocol.WireError{
				Code:    protocol.ErrorCodeMalformedFrame,
				Message: reason,
			},
		}
	}

	job := &store.Job{
		ID:                  upsert.JobUUID,
		Key:                 store.JobKey(upsert.JobKey),
		ExecutorType:        upsert.ExecutorType,
		Function:            upsert.Function,
		Args:                store.Payload(upsert.Args),
		Schedule:            upsert.Schedule,
		Enabled:             store.Enabled(upsert.Enabled),
		Backfill:            upsert.Backfill,
		Retry:               upsert.Retry,
		Overlap:             upsert.Overlap,
		Pin:                 upsert.Pin,
		SubmitterInstanceID: instanceID,
	}

	stored, err := e.jobs.UpsertJob(ctx, job)
	if err != nil {
		e.log.Errorf(logTag+" job upsert failed: %v", err)

		return &protocol.JobUpsertAck{
			JobKey:   upsert.JobKey,
			JobUUID:  upsert.JobUUID,
			Accepted: false,
			Error: &protocol.WireError{
				Code:      protocol.ErrorCodeInternal,
				Retryable: true,
				Message:   err.UnwrapLastError(),
			},
		}
	}

	e.cancelReplacedExecutions(ctx, stored)

	if stored.Enabled {
		e.materializeJob(ctx, stored, e.now())
	}

	e.Notify()

	return &protocol.JobUpsertAck{
		JobKey:   upsert.JobKey,
		JobUUID:  stored.ID,
		Accepted: true,
	}
}

func (e *engine) cancelReplacedExecutions(ctx context.Context, job *store.Job) {
	if job.Version <= 1 {
		return
	}

	existing, err := e.executions.ExecutionsForJob(ctx, job.ID)
	if err != nil {
		e.log.Errorf(logTag+" existing executions lookup failed: %v", err)

		return
	}

	for _, execution := range existing {
		if execution.State.Terminal() ||
			execution.State == store.StateDispatching ||
			execution.State == store.StateRunning {
			continue
		}

		if bool(job.Enabled) && scheduleContains(job.Schedule, execution.ScheduledAt) {
			continue
		}

		e.transition(
			ctx,
			execution,
			store.StateCancelled,
			func(update *store.ExecutionUpdate) {
				reason := store.WaitReason(cancelReasonJobReplaced)
				update.WaitReason = &reason
			},
		)
	}
}

func (e *engine) materializeJob(
	ctx context.Context,
	job *store.Job,
	now time.Time,
) {
	mode := e.resolveBackfill(job.Backfill.Mode)

	cappedCount := e.cfg.BackfillMaxCount

	maxCount := store.OccurrenceCount(job.Backfill.MaxCount)
	if maxCount == 0 || (cappedCount > 0 && maxCount > cappedCount) {
		maxCount = cappedCount
	}

	cappedAge := e.cfg.BackfillMaxAge

	maxAge := millisDuration(job.Backfill.MaxAgeMillis)
	if maxAge <= 0 || (cappedAge > 0 && maxAge > cappedAge) {
		maxAge = cappedAge
	}

	missed, skipped := missedOccurrences(job.Schedule, now, maxCount, maxAge)

	if mode == protocol.BackfillModeDisabled {
		skipped += store.OccurrenceCount(len(missed))
		missed = nil
	}

	if skipped > 0 {
		if err := e.jobs.AddSkippedOccurrences(ctx, job.ID, skipped); err != nil {
			e.log.Errorf(logTag+" skipped occurrence recording failed: %v", err)
		}

		e.metrics.BackfillSkipped.Add(uint64(skipped))
	}

	for _, occurrence := range missed {
		_, created, err := e.executions.CreateExecution(
			ctx,
			job.ID,
			occurrence,
			store.StateScheduled,
			true,
		)
		if err != nil {
			e.log.Errorf(logTag+" backfill execution creation failed: %v", err)

			continue
		}

		if created {
			e.metrics.BackfillCreated.Add(1)
		}
	}

	if next, exists := nextOccurrence(job.Schedule, now); exists {
		if _, _, err := e.executions.CreateExecution(
			ctx,
			job.ID,
			next,
			store.StateScheduled,
			false,
		); err != nil {
			e.log.Errorf(logTag+" next occurrence creation failed: %v", err)
		}
	}
}

func (e *engine) resolveBackfill(
	mode protocol.BackfillMode,
) (resolved protocol.BackfillMode) {
	if mode != protocol.BackfillModeInherit {
		return mode
	}

	if e.cfg.DefaultBackfill != protocol.BackfillModeInherit {
		return e.cfg.DefaultBackfill
	}

	return protocol.BackfillModeEnabled
}

func (e *engine) HandleExecAccept(
	ctx context.Context,
	instanceID protocol.InstanceID,
	accept *protocol.ExecAccept,
) {
	execution, err := e.executions.GetExecution(ctx, accept.ExecutionID)
	if err != nil {
		e.metrics.StaleMessages.Add(1)
		e.log.Warnf(logTag+" accept for unknown execution %d", accept.ExecutionID)

		return
	}

	if execution.CurrentAttemptID != accept.AttemptID {
		e.metrics.StaleMessages.Add(1)

		return
	}

	if !e.attemptOwnedBy(ctx, accept.AttemptID, instanceID) {
		e.metrics.StaleMessages.Add(1)
		e.log.Warnf(
			logTag+" accept for attempt %d from foreign instance %s",
			accept.AttemptID,
			instanceID,
		)

		return
	}

	if !accept.Accepted {
		e.settleRejectedAttempt(ctx, instanceID, execution, accept)

		return
	}

	if execution.State != store.StateDispatching {
		e.metrics.StaleMessages.Add(1)

		return
	}

	running := store.StateRunning
	lease := e.now().Add(e.cfg.Lease)
	consumed := execution.FunctionAttempts + 1

	if _, updateErr := e.executions.UpdateExecution(
		ctx,
		execution.ID,
		execution.Version,
		store.ExecutionUpdate{
			State:            &running,
			LeaseExpiresAt:   &lease,
			FunctionAttempts: &consumed,
		},
	); updateErr != nil {
		e.executionLog(execution).Warnf(logTag+" accept transition lost: %v", updateErr)

		return
	}

	if _, attemptErr := e.attempts.UpdateAttemptState(
		ctx,
		accept.AttemptID,
		[]store.AttemptState{store.AttemptDispatched},
		store.AttemptAccepted,
		"",
	); attemptErr != nil {
		e.executionLog(execution).
			Errorf(logTag+" attempt accept update failed: %v", attemptErr)
	}
}

func (e *engine) settleRejectedAttempt(
	ctx context.Context,
	instanceID protocol.InstanceID,
	execution *store.Execution,
	accept *protocol.ExecAccept,
) {
	rejectionText := store.ErrorText(rejectReasonUnknown)
	if accept.Error != nil {
		rejectionText = store.ErrorText(accept.Error.Message)
	}

	rejected, err := e.attempts.UpdateAttemptState(
		ctx,
		accept.AttemptID,
		[]store.AttemptState{store.AttemptDispatched},
		store.AttemptInfraFailed,
		rejectionText,
	)
	if err != nil {
		e.executionLog(execution).Errorf(logTag+" attempt reject update failed: %v", err)

		return
	}

	if !rejected {
		e.metrics.StaleMessages.Add(1)

		return
	}

	if entry, found := e.registry.Get(instanceID); found {
		entry.ReleaseInFlight(accept.AttemptID)
	}

	e.metrics.InfraRedispatches.Add(1)

	if accept.Error != nil &&
		(accept.Error.Code == protocol.ErrorCodeUnknownFunction ||
			accept.Error.Code == protocol.ErrorCodeIncompatibleFunction) {
		e.transition(
			ctx,
			execution,
			store.StateWaitingCompatible,
			func(update *store.ExecutionUpdate) {
				reason := store.WaitReason(rejectionText)
				update.WaitReason = &reason
			},
		)

		return
	}

	e.requeueReady(ctx, execution.ID, e.cfg.RedispatchDelay)
	e.Notify()
}

func (e *engine) HandleExecResult(
	ctx context.Context,
	instanceID protocol.InstanceID,
	result *protocol.ExecResult,
) {
	execution, err := e.executions.GetExecution(ctx, result.ExecutionID)
	if err != nil {
		e.metrics.StaleMessages.Add(1)
		e.log.Warnf(logTag+" result for unknown execution %d", result.ExecutionID)

		return
	}

	if execution.CurrentAttemptID != result.AttemptID ||
		execution.State != store.StateRunning {
		e.metrics.StaleMessages.Add(1)

		return
	}

	if !e.attemptOwnedBy(ctx, result.AttemptID, instanceID) {
		e.metrics.StaleMessages.Add(1)
		e.log.Warnf(
			logTag+" result for attempt %d from foreign instance %s",
			result.AttemptID,
			instanceID,
		)

		return
	}

	if entry, found := e.registry.Get(instanceID); found {
		entry.ReleaseInFlight(result.AttemptID)
	}

	if result.Success {
		e.settleSuccess(ctx, execution, result)

		return
	}

	e.settleFailure(ctx, execution, result)
}

func (e *engine) settleSuccess(
	ctx context.Context,
	execution *store.Execution,
	result *protocol.ExecResult,
) {
	succeeded := store.StateSucceeded

	if _, err := e.executions.UpdateExecution(
		ctx,
		execution.ID,
		execution.Version,
		store.ExecutionUpdate{State: &succeeded},
	); err != nil {
		e.executionLog(execution).Warnf(logTag+" success transition lost: %v", err)

		return
	}

	if _, err := e.attempts.UpdateAttemptState(
		ctx,
		result.AttemptID,
		[]store.AttemptState{store.AttemptAccepted},
		store.AttemptSucceeded,
		"",
	); err != nil {
		e.executionLog(execution).Errorf(logTag+" attempt success update failed: %v", err)
	}

	e.metrics.FunctionSuccesses.Add(1)
	e.executionLog(execution).Info(logTag + " execution succeeded")
}

func (e *engine) settleFailure(
	ctx context.Context,
	execution *store.Execution,
	result *protocol.ExecResult,
) {
	failureText := store.ErrorText(failureReasonUnknown)
	retryable := false

	if result.Error != nil {
		failureText = store.ErrorText(result.Error.Message)
		retryable = result.Error.Retryable
	}

	if _, err := e.attempts.UpdateAttemptState(
		ctx,
		result.AttemptID,
		[]store.AttemptState{store.AttemptAccepted},
		store.AttemptFunctionFailed,
		failureText,
	); err != nil {
		e.executionLog(execution).Errorf(logTag+" attempt failure update failed: %v", err)
	}

	job, jobErr := e.jobs.GetJob(ctx, execution.JobID)
	if jobErr != nil {
		e.executionLog(execution).Errorf(logTag+" job lookup failed: %v", jobErr)

		return
	}

	if retryable && execution.FunctionAttempts < maxFunctionAttempts(job.Retry) {
		retryWait := store.StateRetryWait
		next := e.now().Add(retryDelay(job.Retry, execution.FunctionAttempts, &e.cfg))

		if _, err := e.executions.UpdateExecution(
			ctx,
			execution.ID,
			execution.Version,
			store.ExecutionUpdate{
				State:         &retryWait,
				NextAttemptAt: &next,
				LastError:     &failureText,
			},
		); err != nil {
			e.executionLog(execution).Warnf(logTag+" retry transition lost: %v", err)

			return
		}

		e.metrics.FunctionRetries.Add(1)
		e.Notify()
		e.executionLog(execution).Warnf(
			logTag+" function failed, retry %d scheduled: %s",
			execution.FunctionAttempts,
			failureText,
		)

		return
	}

	failed := store.StateFailed

	if _, err := e.executions.UpdateExecution(
		ctx,
		execution.ID,
		execution.Version,
		store.ExecutionUpdate{State: &failed, LastError: &failureText},
	); err != nil {
		e.executionLog(execution).Warnf(logTag+" failure transition lost: %v", err)

		return
	}

	e.metrics.FunctionFailures.Add(1)
	e.executionLog(execution).
		Errorf(logTag+" execution failed permanently: %s", failureText)
}

func (e *engine) HandleDisconnect(
	ctx context.Context,
	instanceID protocol.InstanceID,
) {
	open, err := e.attempts.AttemptsOnInstance(
		ctx,
		instanceID,
		store.AttemptDispatched,
		store.AttemptAccepted,
	)
	if err != nil {
		e.log.Errorf(logTag+" open attempts lookup failed: %v", err)

		return
	}

	requeued := false

	for _, attempt := range open {
		execution, execErr := e.executions.GetExecution(ctx, attempt.ExecutionID)
		if execErr != nil {
			continue
		}

		if execution.CurrentAttemptID != attempt.ID {
			continue
		}

		if execution.State != store.StateDispatching &&
			execution.State != store.StateRunning {
			continue
		}

		e.abandonAttempt(ctx, execution, lostReasonDisconnect)

		requeued = true
	}

	if requeued {
		e.Notify()
	}
}

func (e *engine) HandleHeartbeat(
	ctx context.Context,
	instanceID protocol.InstanceID,
) {
	open, err := e.attempts.AttemptsOnInstance(
		ctx,
		instanceID,
		store.AttemptDispatched,
		store.AttemptAccepted,
	)
	if err != nil {
		e.log.Errorf(logTag+" heartbeat attempt lookup failed: %v", err)

		return
	}

	now := e.now()
	lease := now.Add(e.cfg.Lease)

	for _, attempt := range open {
		if now.Sub(attempt.CreatedAt) >= e.cfg.MaxExecution {
			continue
		}

		execution, execErr := e.executions.GetExecution(ctx, attempt.ExecutionID)
		if execErr != nil {
			continue
		}

		if execution.CurrentAttemptID != attempt.ID {
			continue
		}

		if execution.State != store.StateDispatching &&
			execution.State != store.StateRunning {
			continue
		}

		if _, updateErr := e.executions.UpdateExecution(
			ctx,
			execution.ID,
			execution.Version,
			store.ExecutionUpdate{LeaseExpiresAt: &lease},
		); updateErr != nil {
			e.executionLog(execution).Debugf(logTag+" lease renewal lost: %v", updateErr)
		}
	}
}

func (e *engine) attemptOwnedBy(
	ctx context.Context,
	attemptID protocol.AttemptID,
	instanceID protocol.InstanceID,
) (owned bool) {
	attempt, err := e.attempts.GetAttempt(ctx, attemptID)
	if err != nil {
		return false
	}

	return attempt.InstanceID == instanceID
}

func (e *engine) reconsiderWaiting(ctx context.Context, change RegistryChange) {
	waiting, err := e.executions.ExecutionsInStates(
		ctx,
		store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
	)
	if err != nil {
		e.log.Errorf(logTag+" waiting executions lookup failed: %v", err)

		return
	}

	requeued := false

	for _, execution := range waiting {
		job, jobErr := e.jobs.GetJob(ctx, execution.JobID)
		if jobErr != nil || job.ExecutorType != change.ExecutorType {
			continue
		}

		if execution.State == store.StateWaitingLabel &&
			!slices.Contains(change.Labels, job.Pin.Label) {
			continue
		}

		if e.requeueReady(ctx, execution.ID, 0) {
			requeued = true
		}
	}

	if requeued {
		e.Notify()
	}
}

func (e *engine) recoverOnStartup(ctx context.Context) {
	now := e.now()

	jobs, err := e.jobs.ListEnabledJobs(ctx)
	if err != nil {
		e.log.Errorf(logTag+" startup job listing failed: %v", err)
	} else {
		for _, job := range jobs {
			e.materializeJob(ctx, job, now)
		}
	}

	interrupted, err := e.executions.ExecutionsInStates(
		ctx,
		store.StateDispatching,
		store.StateRunning,
	)
	if err != nil {
		e.log.Errorf(logTag+" startup interrupted lookup failed: %v", err)

		return
	}

	for _, execution := range interrupted {
		e.abandonAttempt(ctx, execution, lostReasonRestart)
	}

	e.Notify()
}

func validateUpsert(upsert *protocol.JobUpsert) (reason string, valid bool) {
	if upsert.JobUUID.IsZero() {
		return upsertReasonZeroJobUUID, false
	}

	if upsert.JobKey == "" {
		return upsertReasonEmptyKey, false
	}

	if upsert.ExecutorType == "" {
		return upsertReasonEmptyType, false
	}

	if upsert.Function.Name == "" {
		return upsertReasonEmptyName, false
	}

	switch upsert.Schedule.Kind {
	case protocol.ScheduleKindOneShot:
		return "", true
	case protocol.ScheduleKindFixedInterval:
		if upsert.Schedule.IntervalMillis == 0 {
			return upsertReasonZeroInterval, false
		}

		if upsert.Schedule.IntervalMillis > maxIntervalMillis {
			return upsertReasonWideInterval, false
		}

		return "", true
	default:
		return upsertReasonUnknownKind, false
	}
}
