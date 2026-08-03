package memstore_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/memstore"
)

const (
	baseUnixSeconds   = 1_700_000_000
	testExecutorType  = protocol.ExecutorType("worker")
	otherExecutorType = protocol.ExecutorType("mailer")
	testFunctionName  = protocol.FunctionName("report")
	testInstanceID    = protocol.InstanceID("exec-1")
	otherInstanceID   = protocol.InstanceID("exec-2")
	noExclusion       = protocol.ExecutionID(0)
	firstAttempt      = store.AttemptNumber(1)
	unlimited         = store.BatchLimit(0)
)

var baseTime = time.Unix(baseUnixSeconds, 0).UTC()

func jobUUID(key store.JobKey) protocol.JobUUID {
	sum := sha256.Sum256([]byte(key))

	var id protocol.JobUUID

	copy(id[:], sum[:])

	return id
}

func newStore(t *testing.T) *memstore.Store {
	t.Helper()

	return newBoundedStore(t, memstore.Config{})
}

func newBoundedStore(t *testing.T, config memstore.Config) *memstore.Store {
	t.Helper()

	memStore := memstore.NewStore(config)
	memStore.SetClock(func() time.Time { return baseTime })

	return memStore
}

func createJob(t *testing.T, memStore *memstore.Store, key store.JobKey) *store.Job {
	t.Helper()

	job, err := memStore.UpsertJob(context.Background(), &store.Job{
		ID:           jobUUID(key),
		Key:          key,
		ExecutorType: testExecutorType,
		Function:     protocol.FunctionSpec{Name: testFunctionName},
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("job creation should not fail: %v", err)
	}

	return job
}

func createExecution(
	t *testing.T,
	memStore *memstore.Store,
	jobID protocol.JobUUID,
	scheduledAt time.Time,
) *store.Execution {
	t.Helper()

	execution, created, err := memStore.CreateExecution(
		context.Background(),
		jobID,
		scheduledAt,
		store.StateScheduled,
		false,
	)
	if err != nil {
		t.Fatalf("execution creation should not fail: %v", err)
	}

	if !created {
		t.Fatal("a fresh occurrence should create an execution")
	}

	return execution
}

func updateExecution(
	t *testing.T,
	memStore *memstore.Store,
	current *store.Execution,
	update store.ExecutionUpdate,
) *store.Execution {
	t.Helper()

	next, err := memStore.UpdateExecution(
		context.Background(),
		current.ID,
		current.Version,
		update,
	)
	if err != nil {
		t.Fatalf("execution update should not fail: %v", err)
	}

	return next
}

func driveState(
	t *testing.T,
	memStore *memstore.Store,
	execution *store.Execution,
	states ...store.ExecutionState,
) *store.Execution {
	t.Helper()

	current := execution

	for _, state := range states {
		current = updateExecution(t, memStore, current, store.ExecutionUpdate{State: &state})
	}

	return current
}

func createAttempt(
	t *testing.T,
	memStore *memstore.Store,
	executionID protocol.ExecutionID,
	number store.AttemptNumber,
	instanceID protocol.InstanceID,
) *store.Attempt {
	t.Helper()

	attempt, err := memStore.CreateAttempt(context.Background(), executionID, number, instanceID)
	if err != nil {
		t.Fatalf("attempt creation should not fail: %v", err)
	}

	return attempt
}

func storeResult(
	t *testing.T,
	memStore *memstore.Store,
	key store.JobKey,
	instanceID protocol.InstanceID,
) (protocol.JobUUID, bool) {
	t.Helper()

	id := jobUUID(key)

	stored, err := memStore.StoreResult(context.Background(), &store.PendingResult{
		JobUUID:    id,
		InstanceID: instanceID,
		Success:    true,
		HasValue:   false,
	})
	if err != nil {
		t.Fatalf("result storage should not fail: %v", err)
	}

	return id, stored
}

func TestUpsertJob(t *testing.T) {
	t.Parallel()

	t.Run(
		"when a new key is upserted / then a job is created at version one",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey       = store.JobKey("job-create")
				firstVersion = store.Version(1)
			)

			memStore := newStore(t)

			job := createJob(t, memStore, jobKey)

			if job.ID.IsZero() {
				t.Error("a created job should keep its minted id")
			}

			if job.ID != jobUUID(jobKey) {
				t.Errorf("a created job should keep the client id: got %s", job.ID)
			}

			if job.Version != firstVersion {
				t.Errorf(
					"a created job should start at version %d, got %d",
					firstVersion,
					job.Version,
				)
			}

			if !job.CreatedAt.Equal(baseTime) {
				t.Errorf("creation time should come from the clock: got %v", job.CreatedAt)
			}
		},
	)

	t.Run(
		"when the same key is upserted again / then identity and counters survive",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey        = store.JobKey("job-reupsert")
				skippedCount  = store.OccurrenceCount(7)
				secondVersion = store.Version(2)
			)

			memStore := newStore(t)
			laterTime := baseTime.Add(time.Hour)

			first := createJob(t, memStore, jobKey)

			if err := memStore.AddSkippedOccurrences(
				context.Background(),
				first.ID,
				skippedCount,
			); err != nil {
				t.Fatalf("recording skipped occurrences should not fail: %v", err)
			}

			memStore.SetClock(func() time.Time { return laterTime })

			second, err := memStore.UpsertJob(context.Background(), &store.Job{
				ID:           jobUUID("job-reupsert-other"),
				Key:          jobKey,
				ExecutorType: testExecutorType,
				Function:     protocol.FunctionSpec{Name: testFunctionName},
				Enabled:      false,
			})
			if err != nil {
				t.Fatalf("re-upserting the same key should not fail: %v", err)
			}

			if second.ID != first.ID {
				t.Errorf("re-upsert should keep the job id: got %s, want %s", second.ID, first.ID)
			}

			if second.Version != secondVersion {
				t.Errorf(
					"re-upsert should bump the version to %d, got %d",
					secondVersion,
					second.Version,
				)
			}

			if !second.CreatedAt.Equal(first.CreatedAt) {
				t.Errorf("re-upsert should preserve creation time: got %v", second.CreatedAt)
			}

			if second.SkippedOccurrences != skippedCount {
				t.Errorf(
					"re-upsert should preserve skipped occurrences: got %d, want %d",
					second.SkippedOccurrences,
					skippedCount,
				)
			}

			if !second.UpdatedAt.Equal(laterTime) {
				t.Errorf("re-upsert should refresh update time: got %v", second.UpdatedAt)
			}
		},
	)

	t.Run(
		"when the same key is upserted under two executor types / then two jobs coexist",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey       = store.JobKey("job-shared-key")
				otherIDSeed  = store.JobKey("job-shared-key-other")
				firstVersion = store.Version(1)
			)

			memStore := newStore(t)

			first := createJob(t, memStore, jobKey)

			second, err := memStore.UpsertJob(context.Background(), &store.Job{
				ID:           jobUUID(otherIDSeed),
				Key:          jobKey,
				ExecutorType: otherExecutorType,
				Function:     protocol.FunctionSpec{Name: testFunctionName},
				Enabled:      true,
			})
			if err != nil {
				t.Fatalf("upserting the key under another executor type should not fail: %v", err)
			}

			if second.ID == first.ID {
				t.Error("another executor type should create its own job")
			}

			if second.ID != jobUUID(otherIDSeed) {
				t.Errorf("the second job should keep its own minted id: got %s", second.ID)
			}

			if second.Version != firstVersion {
				t.Errorf(
					"the second job should start at version %d, got %d",
					firstVersion,
					second.Version,
				)
			}

			firstStored, err := memStore.GetJobByKey(
				context.Background(),
				testExecutorType,
				jobKey,
			)
			if err != nil {
				t.Fatalf("the first job should stay addressable: %v", err)
			}

			if firstStored.ID != first.ID {
				t.Errorf(
					"the first executor type should keep its job: got %s, want %s",
					firstStored.ID,
					first.ID,
				)
			}

			secondStored, err := memStore.GetJobByKey(
				context.Background(),
				otherExecutorType,
				jobKey,
			)
			if err != nil {
				t.Fatalf("the second job should be addressable: %v", err)
			}

			if secondStored.ID != second.ID {
				t.Errorf(
					"the second executor type should keep its job: got %s, want %s",
					secondStored.ID,
					second.ID,
				)
			}
		},
	)

	t.Run(
		"when the job id is the zero uuid / then the upsert is refused",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-zero-uuid")

			memStore := newStore(t)

			_, err := memStore.UpsertJob(context.Background(), &store.Job{
				Key:          jobKey,
				ExecutorType: testExecutorType,
				Enabled:      true,
			})
			if err == nil {
				t.Fatal("a zero job uuid must not be stored")
			}

			if !errors.Is(err, store.ErrZeroJobUUID) {
				t.Errorf("a zero job uuid should report its own error: %v", err)
			}
		},
	)
}

func TestGetJobByKey(t *testing.T) {
	t.Parallel()

	t.Run(
		"when a stored key is fetched under its executor type / then the job is returned",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-key-hit")

			memStore := newStore(t)

			created := createJob(t, memStore, jobKey)

			fetched, err := memStore.GetJobByKey(context.Background(), testExecutorType, jobKey)
			if err != nil {
				t.Fatalf("a stored key should be found under its executor type: %v", err)
			}

			if fetched.ID != created.ID {
				t.Errorf(
					"the fetch should return the stored job: got %s, want %s",
					fetched.ID,
					created.ID,
				)
			}
		},
	)

	t.Run(
		"when a stored key is fetched under another executor type / then the lookup misses",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-key-miss")

			memStore := newStore(t)

			createJob(t, memStore, jobKey)

			_, err := memStore.GetJobByKey(context.Background(), otherExecutorType, jobKey)
			if err == nil {
				t.Fatal("a lookup under the wrong executor type must miss")
			}

			if !errors.Is(err, store.ErrJobNotFound) {
				t.Errorf("the miss should report its own error: %v", err)
			}
		},
	)
}

func TestCreateExecution(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the same occurrence is created twice / then the first execution is returned",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-dedup")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			scheduledAt := baseTime.Add(time.Minute)

			first := createExecution(t, memStore, job.ID, scheduledAt)

			second, created, err := memStore.CreateExecution(
				context.Background(),
				job.ID,
				scheduledAt,
				store.StateScheduled,
				false,
			)
			if err != nil {
				t.Fatalf("duplicate occurrence creation should not fail: %v", err)
			}

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
}

func TestUpdateExecution(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the expected version is stale / then the update conflicts",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-cas")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)

			ready := store.StateReady

			result, err := memStore.UpdateExecution(
				context.Background(),
				execution.ID,
				execution.Version+1,
				store.ExecutionUpdate{State: &ready},
			)
			if err == nil {
				t.Fatal("a stale expected version must not update")
			}

			if !errors.Is(err, store.ErrVersionConflict) {
				t.Errorf("a stale version should report a version conflict: %v", err)
			}

			if result != nil {
				t.Error("a conflicting update should not return an execution")
			}
		},
	)

	t.Run(
		"when the execution is terminal / then a state change is refused",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-terminal")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)
			cancelled := driveState(t, memStore, execution, store.StateCancelled)

			ready := store.StateReady

			_, err := memStore.UpdateExecution(
				context.Background(),
				cancelled.ID,
				cancelled.Version,
				store.ExecutionUpdate{State: &ready},
			)
			if err == nil {
				t.Fatal("a terminal execution must not change state")
			}

			if !errors.Is(err, store.ErrTerminalState) {
				t.Errorf("a terminal execution should report a terminal state error: %v", err)
			}
		},
	)

	t.Run(
		"when the transition is not in the table / then the update is refused",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-illegal")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)

			running := store.StateRunning

			_, err := memStore.UpdateExecution(
				context.Background(),
				execution.ID,
				execution.Version,
				store.ExecutionUpdate{State: &running},
			)
			if err == nil {
				t.Fatal("a scheduled execution must not jump straight to running")
			}

			if !errors.Is(err, store.ErrIllegalTransition) {
				t.Errorf("an illegal transition should report a transition error: %v", err)
			}
		},
	)

	t.Run(
		"when single fields are updated in sequence / then both fields survive",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey    = store.JobKey("job-granular")
				lastError = store.ErrorText("boom")
			)

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)
			nextAttemptAt := baseTime.Add(time.Minute)

			afterNext := updateExecution(t, memStore, execution, store.ExecutionUpdate{
				NextAttemptAt: &nextAttemptAt,
			})

			errorText := lastError
			afterError := updateExecution(t, memStore, afterNext, store.ExecutionUpdate{
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
		},
	)

	t.Run(
		"when the execution already succeeded / then late attempts cannot complete it again",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-late")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)
			succeeded := driveState(
				t,
				memStore,
				execution,
				store.StateDispatching,
				store.StateRunning,
				store.StateSucceeded,
			)

			failed := store.StateFailed

			_, staleErr := memStore.UpdateExecution(
				context.Background(),
				succeeded.ID,
				succeeded.Version-1,
				store.ExecutionUpdate{State: &failed},
			)
			if staleErr == nil {
				t.Fatal("an update with a stale version must not complete a settled execution")
			}

			if !errors.Is(staleErr, store.ErrVersionConflict) {
				t.Errorf("a stale late update should report a version conflict: %v", staleErr)
			}

			_, freshErr := memStore.UpdateExecution(
				context.Background(),
				succeeded.ID,
				succeeded.Version,
				store.ExecutionUpdate{State: &failed},
			)
			if freshErr == nil {
				t.Fatal("a fresh update must not re-complete a settled execution")
			}

			if !errors.Is(freshErr, store.ErrTerminalState) {
				t.Errorf("a fresh late update should report a terminal state error: %v", freshErr)
			}
		},
	)
}

func TestDueExecutions(t *testing.T) {
	t.Parallel()

	t.Run(
		"when scheduled executions are compared to now / then only past and present are due",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-due-scheduled")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)

			past := createExecution(t, memStore, job.ID, baseTime.Add(-time.Minute))
			boundary := createExecution(t, memStore, job.ID, baseTime)
			createExecution(t, memStore, job.ID, baseTime.Add(time.Minute))

			due, err := memStore.DueExecutions(context.Background(), baseTime, unlimited)
			if err != nil {
				t.Fatalf("due lookup should not fail: %v", err)
			}

			if len(due) != 2 {
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

			const jobKey = store.JobKey("job-due-next")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			futureAttemptAt := baseTime.Add(time.Hour)
			pastAttemptAt := baseTime.Add(-time.Second)

			ready := store.StateReady
			retryWait := store.StateRetryWait

			readyLater := createExecution(t, memStore, job.ID, baseTime.Add(-4*time.Minute))
			updateExecution(t, memStore, readyLater, store.ExecutionUpdate{
				State:         &ready,
				NextAttemptAt: &futureAttemptAt,
			})

			readyNow := createExecution(t, memStore, job.ID, baseTime.Add(-3*time.Minute))
			updateExecution(t, memStore, readyNow, store.ExecutionUpdate{
				State:         &ready,
				NextAttemptAt: &pastAttemptAt,
			})

			retryLater := driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			updateExecution(t, memStore, retryLater, store.ExecutionUpdate{
				State:         &retryWait,
				NextAttemptAt: &futureAttemptAt,
			})

			retryNow := driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			updateExecution(t, memStore, retryNow, store.ExecutionUpdate{
				State:         &retryWait,
				NextAttemptAt: &pastAttemptAt,
			})

			due, err := memStore.DueExecutions(context.Background(), baseTime, unlimited)
			if err != nil {
				t.Fatalf("due lookup should not fail: %v", err)
			}

			if len(due) != 2 {
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

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			scheduledAt := baseTime.Add(-time.Hour)

			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt),
				store.StateWaitingExecutor,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(time.Second)),
				store.StateWaitingCompatible,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(2*time.Second)),
				store.StateDispatching,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(3*time.Second)),
				store.StateDispatching,
				store.StateRunning,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(4*time.Second)),
				store.StateDispatching,
				store.StateRunning,
				store.StateSucceeded,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(5*time.Second)),
				store.StateDispatching,
				store.StateRunning,
				store.StateFailed,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(6*time.Second)),
				store.StateCancelled,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(7*time.Second)),
				store.StateSkipped,
			)
			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, scheduledAt.Add(8*time.Second)),
				store.StateWaitingLabel,
			)

			due, err := memStore.DueExecutions(context.Background(), baseTime, unlimited)
			if err != nil {
				t.Fatalf("due lookup should not fail: %v", err)
			}

			if len(due) != 0 {
				t.Errorf("waiting, leased, and settled executions should never be due: got %v", due)
			}
		},
	)

	t.Run(
		"when due executions are listed / then they order by schedule time then id",
		func(t *testing.T) {
			t.Parallel()

			const (
				laterJobKey   = store.JobKey("job-order-later")
				earlyJobKey   = store.JobKey("job-order-early")
				tiedJobKey    = store.JobKey("job-order-tied")
				takeTwo       = store.BatchLimit(2)
				unlimitedTake = store.BatchLimit(0)
			)

			memStore := newStore(t)
			laterJob := createJob(t, memStore, laterJobKey)
			earlyJob := createJob(t, memStore, earlyJobKey)
			tiedJob := createJob(t, memStore, tiedJobKey)

			tiedAt := baseTime.Add(-2 * time.Minute)

			firstTied := createExecution(t, memStore, laterJob.ID, tiedAt)
			earliest := createExecution(t, memStore, earlyJob.ID, baseTime.Add(-3*time.Minute))
			secondTied := createExecution(t, memStore, tiedJob.ID, tiedAt)

			due, err := memStore.DueExecutions(context.Background(), baseTime, unlimitedTake)
			if err != nil {
				t.Fatalf("due lookup should not fail: %v", err)
			}

			if len(due) != 3 {
				t.Fatalf("all three executions should be due: got %d", len(due))
			}

			if due[0].ID != earliest.ID ||
				due[1].ID != firstTied.ID ||
				due[2].ID != secondTied.ID {
				t.Errorf("due list should order by schedule time then id: got %v", due)
			}

			limited, err := memStore.DueExecutions(context.Background(), baseTime, takeTwo)
			if err != nil {
				t.Fatalf("limited due lookup should not fail: %v", err)
			}

			if len(limited) != int(takeTwo) {
				t.Fatalf("the limit should cap the due list: got %d", len(limited))
			}

			if limited[0].ID != earliest.ID || limited[1].ID != firstTied.ID {
				t.Errorf("the limit should keep the earliest executions: got %v", limited)
			}
		},
	)
}

func TestNextWakeAt(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the store is empty / then no wake time exists",
		func(t *testing.T) {
			t.Parallel()

			memStore := newStore(t)

			_, found, err := memStore.NextWakeAt(context.Background())
			if err != nil {
				t.Fatalf("wake lookup should not fail: %v", err)
			}

			if found {
				t.Error("an empty store should report no wake time")
			}
		},
	)

	t.Run(
		"when scheduled, ready, and retrying executions exist / then the earliest wakes",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-wake")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)

			readyAt := baseTime.Add(time.Minute)
			retryAt := baseTime.Add(2 * time.Minute)

			createExecution(t, memStore, job.ID, baseTime.Add(3*time.Minute))

			ready := store.StateReady
			readyExecution := createExecution(t, memStore, job.ID, baseTime.Add(-time.Minute))
			updateExecution(t, memStore, readyExecution, store.ExecutionUpdate{
				State:         &ready,
				NextAttemptAt: &readyAt,
			})

			retryWait := store.StateRetryWait
			retryExecution := driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			updateExecution(t, memStore, retryExecution, store.ExecutionUpdate{
				State:         &retryWait,
				NextAttemptAt: &retryAt,
			})

			wake, found, err := memStore.NextWakeAt(context.Background())
			if err != nil {
				t.Fatalf("wake lookup should not fail: %v", err)
			}

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

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)

			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime),
				store.StateCancelled,
			)

			_, found, err := memStore.NextWakeAt(context.Background())
			if err != nil {
				t.Fatalf("wake lookup should not fail: %v", err)
			}

			if found {
				t.Error("settled executions should not report a wake time")
			}
		},
	)
}

func TestWaitingLabelIsNotDueAndDoesNotWake(t *testing.T) {
	t.Parallel()

	t.Run(
		"when an overdue execution waits for a label / then it is neither due nor waking",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-waiting-label")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)

			overdueAt := baseTime.Add(-time.Hour)
			nextAttemptAt := baseTime.Add(-time.Minute)

			waitingLabel := store.StateWaitingLabel
			parked := updateExecution(
				t,
				memStore,
				createExecution(t, memStore, job.ID, overdueAt),
				store.ExecutionUpdate{
					State:         &waitingLabel,
					NextAttemptAt: &nextAttemptAt,
				},
			)

			if parked.State != store.StateWaitingLabel {
				t.Fatalf("the execution should be parked on a label: got %s", parked.State)
			}

			due, err := memStore.DueExecutions(context.Background(), baseTime, unlimited)
			if err != nil {
				t.Fatalf("due lookup should not fail: %v", err)
			}

			if len(due) != 0 {
				t.Errorf("an execution waiting for a label should never be due: got %v", due)
			}

			_, found, err := memStore.NextWakeAt(context.Background())
			if err != nil {
				t.Fatalf("wake lookup should not fail: %v", err)
			}

			if found {
				t.Error("an execution waiting for a label should not report a wake time")
			}
		},
	)

	t.Run(
		"when a label arrives / then the parked execution can move on",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-waiting-label-release")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)

			waitingLabel := store.StateWaitingLabel
			parked := updateExecution(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-time.Hour)),
				store.ExecutionUpdate{State: &waitingLabel},
			)

			released := driveState(t, memStore, parked, store.StateReady)

			if released.State != store.StateReady {
				t.Fatalf("a released execution should be ready: got %s", released.State)
			}

			due, err := memStore.DueExecutions(context.Background(), baseTime, unlimited)
			if err != nil {
				t.Fatalf("due lookup should not fail: %v", err)
			}

			if len(due) != 1 || due[0].ID != released.ID {
				t.Errorf("a released execution should become due: got %v", due)
			}
		},
	)
}

func TestExpiredLeases(t *testing.T) {
	t.Parallel()

	t.Run(
		"when leases are compared to now / then only elapsed leased executions return",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-lease")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)

			elapsedLease := baseTime.Add(-time.Second)
			boundaryLease := baseTime
			liveLease := baseTime.Add(time.Minute)

			dispatching := store.StateDispatching
			running := store.StateRunning

			expiredDispatch := createExecution(t, memStore, job.ID, baseTime.Add(-4*time.Minute))
			updateExecution(t, memStore, expiredDispatch, store.ExecutionUpdate{
				State:          &dispatching,
				LeaseExpiresAt: &elapsedLease,
			})

			boundaryRunning := driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-3*time.Minute)),
				store.StateDispatching,
			)
			updateExecution(t, memStore, boundaryRunning, store.ExecutionUpdate{
				State:          &running,
				LeaseExpiresAt: &boundaryLease,
			})

			liveRunning := driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
			)
			updateExecution(t, memStore, liveRunning, store.ExecutionUpdate{
				State:          &running,
				LeaseExpiresAt: &liveLease,
			})

			createExecution(t, memStore, job.ID, baseTime.Add(-time.Minute))

			expired, err := memStore.ExpiredLeases(context.Background(), baseTime)
			if err != nil {
				t.Fatalf("expired lease lookup should not fail: %v", err)
			}

			if len(expired) != 2 {
				t.Fatalf("only elapsed leases should return: got %d", len(expired))
			}

			if expired[0].ID != expiredDispatch.ID || expired[1].ID != boundaryRunning.ID {
				t.Errorf("elapsed and boundary leases should return in order: got %v", expired)
			}
		},
	)
}

func TestHasActiveExecution(t *testing.T) {
	t.Parallel()

	t.Run(
		"when a sibling execution runs / then activity excludes the asked execution",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-active")
				emptyJobKey = store.JobKey("job-active-empty")
			)

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			emptyJob := createJob(t, memStore, emptyJobKey)

			runningExecution := driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-2*time.Minute)),
				store.StateDispatching,
				store.StateRunning,
			)
			scheduledExecution := createExecution(t, memStore, job.ID, baseTime.Add(-time.Minute))

			activeBesidesRunning, err := memStore.HasActiveExecution(
				context.Background(),
				job.ID,
				runningExecution.ID,
			)
			if err != nil {
				t.Fatalf("activity lookup should not fail: %v", err)
			}

			if activeBesidesRunning {
				t.Error("the running execution itself should be excluded from activity")
			}

			activeBesidesScheduled, err := memStore.HasActiveExecution(
				context.Background(),
				job.ID,
				scheduledExecution.ID,
			)
			if err != nil {
				t.Fatalf("activity lookup should not fail: %v", err)
			}

			if !activeBesidesScheduled {
				t.Error("a running sibling should count as activity")
			}

			activeElsewhere, err := memStore.HasActiveExecution(
				context.Background(),
				emptyJob.ID,
				noExclusion,
			)
			if err != nil {
				t.Fatalf("activity lookup should not fail: %v", err)
			}

			if activeElsewhere {
				t.Error("a job without executions should report no activity")
			}
		},
	)
}

func TestHasPendingOccurrence(t *testing.T) {
	t.Parallel()

	t.Run(
		"when occurrences settle / then only non-terminal ones count as pending",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-pending")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)

			driveState(
				t,
				memStore,
				createExecution(t, memStore, job.ID, baseTime.Add(-time.Minute)),
				store.StateCancelled,
			)

			pendingAfterCancel, err := memStore.HasPendingOccurrence(context.Background(), job.ID)
			if err != nil {
				t.Fatalf("pending lookup should not fail: %v", err)
			}

			if pendingAfterCancel {
				t.Error("a cancelled occurrence should not count as pending")
			}

			createExecution(t, memStore, job.ID, baseTime.Add(time.Minute))

			pendingWithScheduled, err := memStore.HasPendingOccurrence(context.Background(), job.ID)
			if err != nil {
				t.Fatalf("pending lookup should not fail: %v", err)
			}

			if !pendingWithScheduled {
				t.Error("a scheduled occurrence should count as pending")
			}
		},
	)
}

func TestUpdateAttemptState(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the from states do not match / then the attempt is untouched",
		func(t *testing.T) {
			t.Parallel()

			const jobKey = store.JobKey("job-attempt-from")

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)
			attempt := createAttempt(t, memStore, execution.ID, firstAttempt, testInstanceID)

			updated, err := memStore.UpdateAttemptState(
				context.Background(),
				attempt.ID,
				[]store.AttemptState{store.AttemptAccepted},
				store.AttemptSucceeded,
				"",
			)
			if err != nil {
				t.Fatalf("a mismatched from state should not error: %v", err)
			}

			if updated {
				t.Error("a mismatched from state should not update the attempt")
			}

			fetched, fetchErr := memStore.GetAttempt(context.Background(), attempt.ID)
			if fetchErr != nil {
				t.Fatalf("attempt fetch should not fail: %v", fetchErr)
			}

			if fetched.State != store.AttemptDispatched {
				t.Errorf("a refused update should keep the state: got %d", fetched.State)
			}
		},
	)

	t.Run(
		"when the from state matches / then the attempt transitions and records the error",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey      = store.JobKey("job-attempt-to")
				failureText = store.ErrorText("function exploded")
			)

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)
			attempt := createAttempt(t, memStore, execution.ID, firstAttempt, testInstanceID)

			accepted, err := memStore.UpdateAttemptState(
				context.Background(),
				attempt.ID,
				[]store.AttemptState{store.AttemptDispatched},
				store.AttemptAccepted,
				"",
			)
			if err != nil {
				t.Fatalf("a matching from state should not error: %v", err)
			}

			if !accepted {
				t.Fatal("a matching from state should update the attempt")
			}

			failed, err := memStore.UpdateAttemptState(
				context.Background(),
				attempt.ID,
				nil,
				store.AttemptFunctionFailed,
				failureText,
			)
			if err != nil {
				t.Fatalf("an unconditional update should not error: %v", err)
			}

			if !failed {
				t.Fatal("an unconditional update should apply")
			}

			fetched, fetchErr := memStore.GetAttempt(context.Background(), attempt.ID)
			if fetchErr != nil {
				t.Fatalf("attempt fetch should not fail: %v", fetchErr)
			}

			if fetched.State != store.AttemptFunctionFailed {
				t.Errorf("the attempt should hold the new state: got %d", fetched.State)
			}

			if fetched.Error != failureText {
				t.Errorf("the attempt should record the error text: got %q", fetched.Error)
			}
		},
	)
}

func TestAttemptsOnInstance(t *testing.T) {
	t.Parallel()

	t.Run(
		"when attempts spread across instances / then filters select by instance and state",
		func(t *testing.T) {
			t.Parallel()

			const (
				jobKey        = store.JobKey("job-attempt-instance")
				secondAttempt = store.AttemptNumber(2)
			)

			memStore := newStore(t)
			job := createJob(t, memStore, jobKey)
			execution := createExecution(t, memStore, job.ID, baseTime)

			dispatched := createAttempt(t, memStore, execution.ID, firstAttempt, testInstanceID)
			succeeded := createAttempt(t, memStore, execution.ID, secondAttempt, testInstanceID)
			createAttempt(t, memStore, execution.ID, firstAttempt, otherInstanceID)

			if _, err := memStore.UpdateAttemptState(
				context.Background(),
				succeeded.ID,
				nil,
				store.AttemptSucceeded,
				"",
			); err != nil {
				t.Fatalf("attempt update should not fail: %v", err)
			}

			all, err := memStore.AttemptsOnInstance(context.Background(), testInstanceID)
			if err != nil {
				t.Fatalf("instance lookup should not fail: %v", err)
			}

			if len(all) != 2 {
				t.Fatalf("the instance should hold two attempts: got %d", len(all))
			}

			if all[0].ID != dispatched.ID || all[1].ID != succeeded.ID {
				t.Errorf("an unfiltered lookup should keep creation order: got %v", all)
			}

			onlyDispatched, err := memStore.AttemptsOnInstance(
				context.Background(),
				testInstanceID,
				store.AttemptDispatched,
			)
			if err != nil {
				t.Fatalf("filtered lookup should not fail: %v", err)
			}

			if len(onlyDispatched) != 1 || onlyDispatched[0].ID != dispatched.ID {
				t.Errorf(
					"a state filter should keep matching attempts only: got %v",
					onlyDispatched,
				)
			}

			otherAccepted, err := memStore.AttemptsOnInstance(
				context.Background(),
				otherInstanceID,
				store.AttemptAccepted,
			)
			if err != nil {
				t.Fatalf("filtered lookup should not fail: %v", err)
			}

			if len(otherAccepted) != 0 {
				t.Errorf("a non-matching filter should return nothing: got %v", otherAccepted)
			}
		},
	)
}

func TestStoreResultCapPerInstance(t *testing.T) {
	t.Parallel()

	t.Run(
		"when one instance fills its cap / then further results are refused, not failed",
		func(t *testing.T) {
			t.Parallel()

			const perInstanceCap = store.OccurrenceCount(2)

			memStore := newBoundedStore(t, memstore.Config{
				MaxResultsPerInstance: perInstanceCap,
			})

			if _, stored := storeResult(t, memStore, "result-cap-1", testInstanceID); !stored {
				t.Fatal("the first result should fit the cap")
			}

			if _, stored := storeResult(t, memStore, "result-cap-2", testInstanceID); !stored {
				t.Fatal("the second result should fit the cap")
			}

			_, overflow := storeResult(t, memStore, "result-cap-3", testInstanceID)
			if overflow {
				t.Error("a result past the per-instance cap should be refused")
			}

			count, err := memStore.CountResults(context.Background())
			if err != nil {
				t.Fatalf("result count should not fail: %v", err)
			}

			if count != perInstanceCap {
				t.Errorf("a refused result should not be stored: got %d", count)
			}
		},
	)

	t.Run(
		"when one instance is full / then another instance still stores results",
		func(t *testing.T) {
			t.Parallel()

			const (
				perInstanceCap = store.OccurrenceCount(1)
				bothInstances  = store.OccurrenceCount(2)
			)

			memStore := newBoundedStore(t, memstore.Config{
				MaxResultsPerInstance: perInstanceCap,
			})

			if _, stored := storeResult(t, memStore, "result-split-1", testInstanceID); !stored {
				t.Fatal("the first instance should store its result")
			}

			if _, overflow := storeResult(
				t,
				memStore,
				"result-split-2",
				testInstanceID,
			); overflow {
				t.Error("the first instance should be full")
			}

			if _, stored := storeResult(t, memStore, "result-split-3", otherInstanceID); !stored {
				t.Error("a second instance should have its own budget")
			}

			count, err := memStore.CountResults(context.Background())
			if err != nil {
				t.Fatalf("result count should not fail: %v", err)
			}

			if count != bothInstances {
				t.Errorf("both instances should hold one result each: got %d", count)
			}
		},
	)

	t.Run(
		"when a full instance re-stores a held job / then the replacement is accepted",
		func(t *testing.T) {
			t.Parallel()

			const (
				perInstanceCap = store.OccurrenceCount(1)
				heldJobKey     = store.JobKey("result-replace")
			)

			memStore := newBoundedStore(t, memstore.Config{
				MaxResultsPerInstance: perInstanceCap,
			})

			if _, stored := storeResult(t, memStore, heldJobKey, testInstanceID); !stored {
				t.Fatal("the first result should fit the cap")
			}

			if _, stored := storeResult(t, memStore, heldJobKey, testInstanceID); !stored {
				t.Error("re-storing a held job should not count against the cap")
			}

			count, err := memStore.CountResults(context.Background())
			if err != nil {
				t.Fatalf("result count should not fail: %v", err)
			}

			if count != perInstanceCap {
				t.Errorf("a replacement should not add a record: got %d", count)
			}
		},
	)

	t.Run(
		"when the total cap is reached / then every instance is refused",
		func(t *testing.T) {
			t.Parallel()

			const totalCap = store.OccurrenceCount(1)

			memStore := newBoundedStore(t, memstore.Config{MaxResults: totalCap})

			if _, stored := storeResult(t, memStore, "result-total-1", testInstanceID); !stored {
				t.Fatal("the first result should fit the total cap")
			}

			if _, overflow := storeResult(
				t,
				memStore,
				"result-total-2",
				otherInstanceID,
			); overflow {
				t.Error("a result past the total cap should be refused")
			}

			count, err := memStore.CountResults(context.Background())
			if err != nil {
				t.Fatalf("result count should not fail: %v", err)
			}

			if count != totalCap {
				t.Errorf("the total cap should bound the store: got %d", count)
			}
		},
	)

	t.Run(
		"when a nil result is stored / then the store reports its own error",
		func(t *testing.T) {
			t.Parallel()

			memStore := newStore(t)

			stored, err := memStore.StoreResult(context.Background(), nil)
			if err == nil {
				t.Fatal("a nil pending result must not be stored")
			}

			if stored {
				t.Error("a nil pending result should not report storage")
			}

			if !errors.Is(err, store.ErrNilResult) {
				t.Errorf("a nil pending result should report its own error: %v", err)
			}
		},
	)
}

func TestResultsForInstanceOrdering(t *testing.T) {
	t.Parallel()

	t.Run(
		"when results are stored for one instance / then they return in storage order",
		func(t *testing.T) {
			t.Parallel()

			const takeTwo = store.BatchLimit(2)

			memStore := newStore(t)

			first, _ := storeResult(t, memStore, "result-order-1", testInstanceID)
			second, _ := storeResult(t, memStore, "result-order-2", testInstanceID)
			third, _ := storeResult(t, memStore, "result-order-3", testInstanceID)
			foreign, _ := storeResult(t, memStore, "result-order-4", otherInstanceID)

			held, err := memStore.ResultsForInstance(
				context.Background(),
				testInstanceID,
				unlimited,
			)
			if err != nil {
				t.Fatalf("instance result lookup should not fail: %v", err)
			}

			if len(held) != 3 {
				t.Fatalf("the instance should hold three results: got %d", len(held))
			}

			if held[0].JobUUID != first ||
				held[1].JobUUID != second ||
				held[2].JobUUID != third {
				t.Errorf("results should return in storage order: got %v", held)
			}

			limited, err := memStore.ResultsForInstance(
				context.Background(),
				testInstanceID,
				takeTwo,
			)
			if err != nil {
				t.Fatalf("limited instance lookup should not fail: %v", err)
			}

			if len(limited) != int(takeTwo) {
				t.Fatalf("the limit should cap the result list: got %d", len(limited))
			}

			if limited[0].JobUUID != first || limited[1].JobUUID != second {
				t.Errorf("the limit should keep the earliest results: got %v", limited)
			}

			other, err := memStore.ResultsForInstance(
				context.Background(),
				otherInstanceID,
				unlimited,
			)
			if err != nil {
				t.Fatalf("foreign instance lookup should not fail: %v", err)
			}

			if len(other) != 1 || other[0].JobUUID != foreign {
				t.Errorf("an instance should only see its own results: got %v", other)
			}
		},
	)

	t.Run(
		"when a middle result is deleted / then the surviving order holds",
		func(t *testing.T) {
			t.Parallel()

			memStore := newStore(t)

			first, _ := storeResult(t, memStore, "result-delete-1", testInstanceID)
			second, _ := storeResult(t, memStore, "result-delete-2", testInstanceID)
			third, _ := storeResult(t, memStore, "result-delete-3", testInstanceID)

			deleted, err := memStore.DeleteResult(context.Background(), second)
			if err != nil {
				t.Fatalf("result deletion should not fail: %v", err)
			}

			if !deleted {
				t.Fatal("a held result should report deletion")
			}

			replayed, err := memStore.DeleteResult(context.Background(), second)
			if err != nil {
				t.Fatalf("a replayed deletion should not fail: %v", err)
			}

			if replayed {
				t.Error("a replayed deletion should report nothing deleted")
			}

			held, err := memStore.ResultsForInstance(
				context.Background(),
				testInstanceID,
				unlimited,
			)
			if err != nil {
				t.Fatalf("instance result lookup should not fail: %v", err)
			}

			if len(held) != 2 {
				t.Fatalf("two results should survive the deletion: got %d", len(held))
			}

			if held[0].JobUUID != first || held[1].JobUUID != third {
				t.Errorf("deletion should keep the remaining order: got %v", held)
			}
		},
	)

	t.Run(
		"when a result is marked sent / then its counters advance",
		func(t *testing.T) {
			t.Parallel()

			const firstSend = store.ResultAttempts(1)

			memStore := newStore(t)
			sentAt := baseTime.Add(time.Minute)

			id, _ := storeResult(t, memStore, "result-sent", testInstanceID)

			if err := memStore.MarkResultSent(context.Background(), id, sentAt); err != nil {
				t.Fatalf("marking a held result sent should not fail: %v", err)
			}

			held, err := memStore.ResultsForInstance(
				context.Background(),
				testInstanceID,
				unlimited,
			)
			if err != nil {
				t.Fatalf("instance result lookup should not fail: %v", err)
			}

			if len(held) != 1 {
				t.Fatalf("the instance should hold one result: got %d", len(held))
			}

			if held[0].Attempts != firstSend {
				t.Errorf("a send should count one attempt: got %d", held[0].Attempts)
			}

			if !held[0].LastSentAt.Equal(sentAt) {
				t.Errorf("a send should record its instant: got %v", held[0].LastSentAt)
			}

			missing := jobUUID("result-sent-missing")

			if markErr := memStore.MarkResultSent(
				context.Background(),
				missing,
				sentAt,
			); markErr == nil {
				t.Fatal("marking an unheld result sent must fail")
			} else if !errors.Is(markErr, store.ErrResultNotFound) {
				t.Errorf("an unheld result should report its own error: %v", markErr)
			}
		},
	)
}

func TestExpiredResults(t *testing.T) {
	t.Parallel()

	t.Run(
		"when results are compared to a cutoff / then only older ones return in order",
		func(t *testing.T) {
			t.Parallel()

			const takeOne = store.BatchLimit(1)

			memStore := newStore(t)

			oldest, _ := storeResult(t, memStore, "result-expiry-1", testInstanceID)

			middleTime := baseTime.Add(time.Minute)
			memStore.SetClock(func() time.Time { return middleTime })

			middle, _ := storeResult(t, memStore, "result-expiry-2", otherInstanceID)

			cutoff := baseTime.Add(2 * time.Minute)
			memStore.SetClock(func() time.Time { return cutoff })

			storeResult(t, memStore, "result-expiry-3", testInstanceID)

			expired, err := memStore.ExpiredResults(context.Background(), cutoff, unlimited)
			if err != nil {
				t.Fatalf("expired result lookup should not fail: %v", err)
			}

			if len(expired) != 2 {
				t.Fatalf("only results older than the cutoff should return: got %d", len(expired))
			}

			if expired[0].JobUUID != oldest || expired[1].JobUUID != middle {
				t.Errorf("expired results should order by storage time: got %v", expired)
			}

			limited, err := memStore.ExpiredResults(context.Background(), cutoff, takeOne)
			if err != nil {
				t.Fatalf("limited expired lookup should not fail: %v", err)
			}

			if len(limited) != int(takeOne) {
				t.Fatalf("the limit should cap the expired list: got %d", len(limited))
			}

			if limited[0].JobUUID != oldest {
				t.Errorf("the limit should keep the oldest result: got %v", limited)
			}
		},
	)

	t.Run(
		"when the cutoff precedes every result / then nothing expires",
		func(t *testing.T) {
			t.Parallel()

			memStore := newStore(t)

			storeResult(t, memStore, "result-fresh-1", testInstanceID)
			storeResult(t, memStore, "result-fresh-2", otherInstanceID)

			expired, err := memStore.ExpiredResults(context.Background(), baseTime, unlimited)
			if err != nil {
				t.Fatalf("expired result lookup should not fail: %v", err)
			}

			if len(expired) != 0 {
				t.Errorf("a result stored at the cutoff should not expire: got %v", expired)
			}
		},
	)
}
