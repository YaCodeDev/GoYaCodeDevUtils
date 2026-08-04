package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// TestExecutionRepository runs the execution conformance subtests against
// stores the factory builds: occurrence creation and dedupe, version-checked
// updates, dueness, wake times, state and job filters, activity checks, and
// lease expiry.
func TestExecutionRepository(t *testing.T, factory Factory) {
	t.Helper()

	t.Run(
		"when a fresh occurrence is created / then an execution materializes at version one",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-create")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			scheduledAt := baseTime.Add(time.Minute)

			execution := createExecution(t, sut, job.ID, scheduledAt)

			if execution.JobID != job.ID {
				t.Errorf("the execution should belong to its job: got %s", execution.JobID)
			}

			if !execution.ScheduledAt.Equal(scheduledAt) {
				t.Errorf("the execution should keep its instant: got %v", execution.ScheduledAt)
			}

			if execution.State != store.StateScheduled {
				t.Errorf("the execution should hold the given state: got %s", execution.State)
			}

			if execution.Version != firstVersion {
				t.Errorf(
					"the execution should start at version %d, got %d",
					firstVersion,
					execution.Version,
				)
			}

			if fetched := getExecution(t, sut, execution.ID); fetched.JobID != job.ID {
				t.Errorf("the stored execution should be fetchable: got %s", fetched.JobID)
			}
		},
	)

	t.Run(
		"when the same occurrence is created twice / then the first execution is returned",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-dedup")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			scheduledAt := baseTime.Add(time.Minute)

			first := createExecution(t, sut, job.ID, scheduledAt)

			second, created, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledAt,
				store.StateScheduled,
				false,
			)
			requireNoError(t, err, "duplicate occurrence creation should not fail")

			if created {
				t.Error("a duplicate occurrence should not report a fresh creation")
			}

			if second.ID != first.ID {
				t.Errorf(
					"a duplicate occurrence should return the first execution: got %d, want %d",
					second.ID,
					first.ID,
				)
			}
		},
	)

	t.Run(
		"when an occurrence is created for an unknown job / then the creation misses",
		func(t *testing.T) {
			t.Parallel()

			const unknownSeed = store.JobKey("job-exec-unknown")

			sut := factory(t)

			_, _, err := sut.CreateExecution(
				context.Background(),
				jobUUID(unknownSeed),
				baseTime,
				store.StateScheduled,
				false,
			)
			requireSentinel(t, err, store.ErrJobNotFound, "an unknown job must not materialize")
		},
	)

	t.Run(
		"when an unknown execution is fetched / then the lookup misses",
		func(t *testing.T) {
			t.Parallel()

			const unknownExecution = protocol.ExecutionID(404)

			sut := factory(t)

			_, err := sut.GetExecution(context.Background(), unknownExecution)
			requireSentinel(t, err, store.ErrExecutionNotFound, "an unknown execution must miss")
		},
	)

	t.Run(
		"when the expected version is stale / then the update conflicts",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-cas")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)

			ready := store.StateReady

			result, err := sut.UpdateExecution(
				context.Background(),
				execution.ID,
				execution.Version+1,
				store.ExecutionUpdate{State: &ready},
			)
			requireSentinel(
				t,
				err,
				store.ErrVersionConflict,
				"a stale expected version must not update",
			)

			if result != nil {
				t.Error("a conflicting update should not return an execution")
			}
		},
	)

	t.Run(
		"when the execution is terminal / then a state change is refused",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-terminal")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			cancelled := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime),
				store.StateCancelled,
			)

			ready := store.StateReady

			_, err := sut.UpdateExecution(
				context.Background(),
				cancelled.ID,
				cancelled.Version,
				store.ExecutionUpdate{State: &ready},
			)
			requireSentinel(
				t,
				err,
				store.ErrTerminalState,
				"a terminal execution must not change state",
			)
		},
	)

	t.Run(
		"when the transition is not in the table / then the update is refused",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-illegal")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)

			running := store.StateRunning

			_, err := sut.UpdateExecution(
				context.Background(),
				execution.ID,
				execution.Version,
				store.ExecutionUpdate{State: &running},
			)
			requireSentinel(
				t,
				err,
				store.ErrIllegalTransition,
				"a scheduled execution must not jump straight to running",
			)
		},
	)

	t.Run(
		"when single fields are updated in sequence / then both fields survive",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-exec-granular")
				lastError   = store.ErrorText("boom")
				wantVersion = store.Version(3)
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			nextAttemptAt := baseTime.Add(time.Minute)

			afterNext := updateExecution(t, sut, execution, store.ExecutionUpdate{
				NextAttemptAt: &nextAttemptAt,
			})

			errorText := lastError
			afterError := updateExecution(t, sut, afterNext, store.ExecutionUpdate{
				LastError: &errorText,
			})

			if !afterError.NextAttemptAt.Equal(nextAttemptAt) {
				t.Errorf(
					"a later error update should not clobber the next attempt time: got %v",
					afterError.NextAttemptAt,
				)
			}

			if afterError.LastError != lastError {
				t.Errorf(
					"an earlier time update should not clobber the last error: got %q",
					afterError.LastError,
				)
			}

			if afterError.Version != wantVersion {
				t.Errorf(
					"every applied update should bump the version to %d: got %d",
					wantVersion,
					afterError.Version,
				)
			}
		},
	)

	t.Run(
		"when scheduled executions are compared to now / then only past and present are due",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey  = store.JobKey("job-due-scheduled")
				wantDue = 2
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			past := createExecution(t, sut, job.ID, baseTime.Add(-time.Minute))
			boundary := createExecution(t, sut, job.ID, baseTime)
			createExecution(t, sut, job.ID, baseTime.Add(time.Minute))

			due := dueExecutions(t, sut, baseTime, unlimited)

			if len(due) != wantDue {
				t.Fatalf("only past and boundary executions should be due: got %d", len(due))
			}

			if due[0].ID != past.ID || due[1].ID != boundary.ID {
				t.Errorf("due list should hold the past then the boundary execution: got %v", due)
			}
		},
	)

	t.Run(
		"when executions wait for a next attempt / then dueness follows the next attempt time",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey  = store.JobKey("job-due-next")
				wantDue = 2
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			futureAttemptAt := baseTime.Add(time.Hour)
			pastAttemptAt := baseTime.Add(-time.Second)

			ready := store.StateReady
			retryWait := store.StateRetryWait

			readyLater := createExecution(t, sut, job.ID, baseTime.Add(-4*time.Minute))
			updateExecution(t, sut, readyLater, store.ExecutionUpdate{
				State:         &ready,
				NextAttemptAt: &futureAttemptAt,
			})

			readyNow := createExecution(t, sut, job.ID, baseTime.Add(-3*time.Minute))
			updateExecution(t, sut, readyNow, store.ExecutionUpdate{
				State:         &ready,
				NextAttemptAt: &pastAttemptAt,
			})

			retryLater := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			updateExecution(t, sut, retryLater, store.ExecutionUpdate{
				State:         &retryWait,
				NextAttemptAt: &futureAttemptAt,
			})

			retryNow := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			updateExecution(t, sut, retryNow, store.ExecutionUpdate{
				State:         &retryWait,
				NextAttemptAt: &pastAttemptAt,
			})

			due := dueExecutions(t, sut, baseTime, unlimited)

			if len(due) != wantDue {
				t.Fatalf(
					"only executions whose next attempt has come should be due: got %d",
					len(due),
				)
			}

			if due[0].ID != readyNow.ID || due[1].ID != retryNow.ID {
				t.Errorf("due list should hold the ready then the retrying execution: got %v", due)
			}
		},
	)

	t.Run(
		"when executions wait, run, or are settled / then none of them are due",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-due-never")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			overdueAt := baseTime.Add(-time.Hour)

			sweeps := [][]store.ExecutionState{
				{store.StateWaitingExecutor},
				{store.StateWaitingCompatible},
				{store.StateWaitingLabel},
				{store.StateDispatching},
				{store.StateDispatching, store.StateRunning},
				{store.StateDispatching, store.StateRunning, store.StateSucceeded},
				{store.StateDispatching, store.StateRunning, store.StateFailed},
				{store.StateCancelled},
				{store.StateSkipped},
			}

			for index, sweep := range sweeps {
				scheduledAt := overdueAt.Add(time.Duration(index) * time.Second)
				driveState(t, sut, createExecution(t, sut, job.ID, scheduledAt), sweep...)
			}

			if due := dueExecutions(t, sut, baseTime, unlimited); len(due) != 0 {
				t.Errorf("waiting, leased, and settled executions should never be due: got %v", due)
			}
		},
	)

	t.Run(
		"when due executions are listed / then they order by schedule time then creation",
		func(t *testing.T) {
			t.Parallel()

			const (
				laterJobKey = store.JobKey("job-order-later")
				earlyJobKey = store.JobKey("job-order-early")
				tiedJobKey  = store.JobKey("job-order-tied")
				takeTwo     = store.BatchLimit(2)
				wantDue     = 3
			)

			sut := factory(t)
			laterJob := createJob(t, sut, laterJobKey)
			earlyJob := createJob(t, sut, earlyJobKey)
			tiedJob := createJob(t, sut, tiedJobKey)

			tiedAt := baseTime.Add(-2 * time.Minute)

			firstTied := createExecution(t, sut, laterJob.ID, tiedAt)
			earliest := createExecution(t, sut, earlyJob.ID, baseTime.Add(-3*time.Minute))
			secondTied := createExecution(t, sut, tiedJob.ID, tiedAt)

			due := dueExecutions(t, sut, baseTime, unlimited)

			if len(due) != wantDue {
				t.Fatalf("all three executions should be due: got %d", len(due))
			}

			if due[0].ID != earliest.ID ||
				due[1].ID != firstTied.ID ||
				due[2].ID != secondTied.ID {
				t.Errorf("due list should order by schedule time then creation: got %v", due)
			}

			limited := dueExecutions(t, sut, baseTime, takeTwo)

			if len(limited) != int(takeTwo) {
				t.Fatalf("the limit should cap the due list: got %d", len(limited))
			}

			if limited[0].ID != earliest.ID || limited[1].ID != firstTied.ID {
				t.Errorf("the limit should keep the earliest executions: got %v", limited)
			}
		},
	)

	t.Run(
		"when the store is empty / then no wake time exists",
		func(t *testing.T) {
			t.Parallel()

			sut := factory(t)

			if _, found := nextWakeAt(t, sut); found {
				t.Error("an empty store should report no wake time")
			}
		},
	)

	t.Run(
		"when scheduled, ready, and retrying executions exist / then the earliest wakes",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey         = store.JobKey("job-wake")
				readyDelay     = time.Minute
				retryDelay     = 2 * time.Minute
				scheduledDelay = 3 * time.Minute
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			readyAt := baseTime.Add(readyDelay)
			retryAt := baseTime.Add(retryDelay)

			createExecution(t, sut, job.ID, baseTime.Add(scheduledDelay))

			ready := store.StateReady
			readyExecution := createExecution(t, sut, job.ID, baseTime.Add(-time.Minute))
			updateExecution(t, sut, readyExecution, store.ExecutionUpdate{
				State:         &ready,
				NextAttemptAt: &readyAt,
			})

			retryWait := store.StateRetryWait
			retryExecution := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			updateExecution(t, sut, retryExecution, store.ExecutionUpdate{
				State:         &retryWait,
				NextAttemptAt: &retryAt,
			})

			wake, found := nextWakeAt(t, sut)

			if !found {
				t.Fatal("pending executions should report a wake time")
			}

			if !wake.Equal(readyAt) {
				t.Errorf("the earliest pending time should wake: got %v, want %v", wake, readyAt)
			}
		},
	)

	t.Run(
		"when only settled executions exist / then no wake time exists",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-wake-settled")

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			driveState(t, sut, createExecution(t, sut, job.ID, baseTime), store.StateCancelled)

			if _, found := nextWakeAt(t, sut); found {
				t.Error("settled executions should not report a wake time")
			}
		},
	)

	t.Run(
		"when an overdue execution waits for a label / then it is neither due nor waking",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-waiting-label")

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			overdueAt := baseTime.Add(-time.Hour)
			nextAttemptAt := baseTime.Add(-time.Minute)

			waitingLabel := store.StateWaitingLabel
			updateExecution(
				t,
				sut,
				createExecution(t, sut, job.ID, overdueAt),
				store.ExecutionUpdate{
					State:         &waitingLabel,
					NextAttemptAt: &nextAttemptAt,
				},
			)

			if due := dueExecutions(t, sut, baseTime, unlimited); len(due) != 0 {
				t.Errorf("an execution waiting for a label should never be due: got %v", due)
			}

			if _, found := nextWakeAt(t, sut); found {
				t.Error("an execution waiting for a label should not report a wake time")
			}
		},
	)

	t.Run(
		"when a label arrives / then the parked execution can move on",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-waiting-label-release")

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			waitingLabel := store.StateWaitingLabel
			parked := updateExecution(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-time.Hour)),
				store.ExecutionUpdate{State: &waitingLabel},
			)

			released := driveState(t, sut, parked, store.StateReady)

			if released.State != store.StateReady {
				t.Fatalf("a released execution should be ready: got %s", released.State)
			}

			due := dueExecutions(t, sut, baseTime, unlimited)

			if len(due) != 1 || due[0].ID != released.ID {
				t.Errorf("a released execution should become due: got %v", due)
			}
		},
	)

	t.Run(
		"when executions are filtered by state / then matching executions return in creation order",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey            = store.JobKey("job-state-filter")
				readyOffset       = time.Second
				dispatchingOffset = 2 * time.Second
				runningOffset     = 3 * time.Second
				wantMatching      = 2
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			createExecution(t, sut, job.ID, baseTime)
			readyExecution := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(readyOffset)),
				store.StateReady,
			)
			driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(dispatchingOffset)),
				store.StateDispatching,
			)
			runningExecution := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(runningOffset)),
				store.StateDispatching,
				store.StateRunning,
			)

			matching := executionsInStates(t, sut, store.StateReady, store.StateRunning)

			if len(matching) != wantMatching {
				t.Fatalf("only matching states should return: got %d", len(matching))
			}

			if matching[0].ID != readyExecution.ID || matching[1].ID != runningExecution.ID {
				t.Errorf("matching executions should keep creation order: got %v", matching)
			}
		},
	)

	t.Run(
		"when executions are listed for a job / then only that job's executions return",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey        = store.JobKey("job-exec-list")
				foreignJobKey = store.JobKey("job-exec-list-other")
				wantOwned     = 2
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			foreignJob := createJob(t, sut, foreignJobKey)

			first := createExecution(t, sut, job.ID, baseTime)
			createExecution(t, sut, foreignJob.ID, baseTime)
			second := createExecution(t, sut, job.ID, baseTime.Add(time.Second))

			owned := executionsForJob(t, sut, job.ID)

			if len(owned) != wantOwned {
				t.Fatalf("only the job's executions should return: got %d", len(owned))
			}

			if owned[0].ID != first.ID || owned[1].ID != second.ID {
				t.Errorf("the job's executions should keep creation order: got %v", owned)
			}
		},
	)

	t.Run(
		"when a sibling execution runs / then activity excludes the asked execution",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-active")
				emptyJobKey = store.JobKey("job-active-empty")
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			emptyJob := createJob(t, sut, emptyJobKey)

			runningExecution := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			scheduledExecution := createExecution(t, sut, job.ID, baseTime.Add(-time.Minute))

			if hasActiveExecution(t, sut, job.ID, runningExecution.ID) {
				t.Error("the running execution itself should be excluded from activity")
			}

			if !hasActiveExecution(t, sut, job.ID, scheduledExecution.ID) {
				t.Error("a running sibling should count as activity")
			}

			if hasActiveExecution(t, sut, emptyJob.ID, noExclusion) {
				t.Error("a job without executions should report no activity")
			}
		},
	)

	t.Run(
		"when occurrences settle / then only non-terminal ones count as pending",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-pending")

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-time.Minute)),
				store.StateCancelled,
			)

			if hasPendingOccurrence(t, sut, job.ID) {
				t.Error("a cancelled occurrence should not count as pending")
			}

			createExecution(t, sut, job.ID, baseTime.Add(time.Minute))

			if !hasPendingOccurrence(t, sut, job.ID) {
				t.Error("a scheduled occurrence should count as pending")
			}
		},
	)

	t.Run(
		"when leases are compared to now / then only elapsed leased executions return",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-lease")
				wantExpired = 2
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			elapsedLease := baseTime.Add(-time.Second)
			boundaryLease := baseTime
			liveLease := baseTime.Add(time.Minute)

			dispatching := store.StateDispatching
			running := store.StateRunning

			expiredDispatch := createExecution(t, sut, job.ID, baseTime.Add(-4*time.Minute))
			updateExecution(t, sut, expiredDispatch, store.ExecutionUpdate{
				State:          &dispatching,
				LeaseExpiresAt: &elapsedLease,
			})

			boundaryRunning := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-3*time.Minute)),
				store.StateDispatching,
			)
			updateExecution(t, sut, boundaryRunning, store.ExecutionUpdate{
				State:          &running,
				LeaseExpiresAt: &boundaryLease,
			})

			liveRunning := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
			)
			updateExecution(t, sut, liveRunning, store.ExecutionUpdate{
				State:          &running,
				LeaseExpiresAt: &liveLease,
			})

			createExecution(t, sut, job.ID, baseTime.Add(-time.Minute))

			expired := expiredLeases(t, sut, baseTime)

			if len(expired) != wantExpired {
				t.Fatalf("only elapsed leases should return: got %d", len(expired))
			}

			if expired[0].ID != expiredDispatch.ID || expired[1].ID != boundaryRunning.ID {
				t.Errorf("elapsed and boundary leases should return in order: got %v", expired)
			}
		},
	)

	t.Run(
		"when a settled execution is compared to a cutoff / then only older settled executions expire",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-expiry")

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			settled := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime),
				store.StateCancelled,
			)

			cutoff := settled.UpdatedAt.Add(time.Second)

			expired := expiredExecutions(t, sut, cutoff, unlimited)

			if len(expired) != 1 || expired[0].ID != settled.ID {
				t.Errorf("the settled execution should expire after its cutoff: got %v", expired)
			}

			if fresh := expiredExecutions(t, sut, settled.UpdatedAt, unlimited); len(fresh) != 0 {
				t.Errorf("an execution settled at the cutoff should not expire: got %v", fresh)
			}
		},
	)

	t.Run(
		"when an execution never settles / then it never appears as expired",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey        = store.JobKey("job-exec-expiry-pending")
				farFutureDays = 365 * 24 * time.Hour
			)

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			createExecution(t, sut, job.ID, baseTime)

			if expired := expiredExecutions(
				t,
				sut,
				baseTime.Add(farFutureDays),
				unlimited,
			); len(expired) != 0 {
				t.Errorf("a non-terminal execution must never expire: got %v", expired)
			}
		},
	)

	t.Run(
		"when a settled execution is deleted / then it is gone",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-delete")

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			settled := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime),
				store.StateCancelled,
			)

			if !deleteExecution(t, sut, settled.ID) {
				t.Fatal("a stored execution delete should report true")
			}

			_, err := sut.GetExecution(context.Background(), settled.ID)
			requireSentinel(
				t,
				err,
				store.ErrExecutionNotFound,
				"the deleted execution should be gone",
			)
		},
	)

	t.Run(
		"when an execution is deleted twice / then the second delete reports false",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-delete-twice")

			sut := factory(t)
			job := createJob(t, sut, jobKey)

			settled := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, baseTime),
				store.StateCancelled,
			)

			if !deleteExecution(t, sut, settled.ID) {
				t.Fatal("the first delete should report true")
			}

			if deleteExecution(t, sut, settled.ID) {
				t.Error("a replayed delete should report false")
			}
		},
	)

	t.Run(
		"when an unknown execution is deleted / then the delete reports false",
		func(t *testing.T) {
			t.Parallel()

			const unknownExecution = protocol.ExecutionID(404)

			sut := factory(t)

			if deleteExecution(t, sut, unknownExecution) {
				t.Error("deleting an unknown execution should report false")
			}
		},
	)

	t.Run(
		"when a settled execution's attempts still exist / then deleting the execution succeeds anyway",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-delete-with-attempts")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			execution := createExecution(t, sut, job.ID, baseTime)
			attempt := createAttempt(t, sut, execution.ID, firstAttempt, suiteInstanceID)

			settled := driveState(
				t,
				sut,
				execution,
				store.StateDispatching,
				store.StateRunning,
				store.StateFailed,
			)

			if !deleteExecution(t, sut, settled.ID) {
				t.Fatal("deleting an execution should succeed even with attempts still stored")
			}

			if fetched := getAttempt(t, sut, attempt.ID); fetched.ExecutionID != execution.ID {
				t.Errorf(
					"the store must not cascade-delete attempts on its own: got %+v",
					fetched,
				)
			}
		},
	)

	t.Run(
		"when a deleted execution's occurrence is recreated / then a fresh execution materializes",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-exec-delete-recreate")

			sut := factory(t)
			job := createJob(t, sut, jobKey)
			scheduledAt := baseTime.Add(time.Minute)

			settled := driveState(
				t,
				sut,
				createExecution(t, sut, job.ID, scheduledAt),
				store.StateCancelled,
			)

			if !deleteExecution(t, sut, settled.ID) {
				t.Fatal("the delete should report true")
			}

			fresh, created, err := sut.CreateExecution(
				context.Background(),
				job.ID,
				scheduledAt,
				store.StateScheduled,
				false,
			)
			requireNoError(t, err, "recreating a purged occurrence should not fail")

			if !created {
				t.Fatal("a purged occurrence should materialize a fresh execution")
			}

			if fresh.ID == settled.ID {
				t.Errorf(
					"the fresh execution should mint a new identity: got %d, want different from %d",
					fresh.ID,
					settled.ID,
				)
			}
		},
	)
}
