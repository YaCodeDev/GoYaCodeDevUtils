package storetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

var baseTime = time.Unix(baseUnixSeconds, 0).UTC()

func jobUUID(key store.JobKey) (id protocol.JobUUID) {
	keyLength := len(key)
	if keyLength > keyLengthCap {
		keyLength = keyLengthCap
	}

	id[0] = byte(keyLength)
	copy(id[1:], key)

	return id
}

func newJob(key store.JobKey) (job *store.Job) {
	return &store.Job{
		ID:           jobUUID(key),
		Key:          key,
		ExecutorType: suiteExecutorType,
		Function:     protocol.FunctionSpec{Name: suiteFunctionName},
		Enabled:      true,
	}
}

func requireNoError(t *testing.T, err yaerrors.Error, intent string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %v", intent, err)
	}
}

func requireSentinel(t *testing.T, err yaerrors.Error, sentinel error, intent string) {
	t.Helper()

	if err == nil {
		t.Fatalf("%s: an error is required", intent)
	}

	if !errors.Is(err, sentinel) {
		t.Errorf("%s: got %v", intent, err)
	}
}

func createJob(t *testing.T, sut store.Store, key store.JobKey) (created *store.Job) {
	t.Helper()

	job, err := sut.UpsertJob(context.Background(), newJob(key))
	requireNoError(t, err, "job creation should not fail")

	return job
}

func upsertJob(t *testing.T, sut store.Store, job *store.Job) (upserted *store.Job) {
	t.Helper()

	upserted, err := sut.UpsertJob(context.Background(), job)
	requireNoError(t, err, "job upsert should not fail")

	return upserted
}

func getJob(t *testing.T, sut store.Store, id protocol.JobUUID) (fetched *store.Job) {
	t.Helper()

	fetched, err := sut.GetJob(context.Background(), id)
	requireNoError(t, err, "job fetch should not fail")

	return fetched
}

func getJobByKey(
	t *testing.T,
	sut store.Store,
	executorType protocol.ExecutorType,
	key store.JobKey,
) (fetched *store.Job) {
	t.Helper()

	fetched, err := sut.GetJobByKey(context.Background(), executorType, key)
	requireNoError(t, err, "job key fetch should not fail")

	return fetched
}

func deleteJob(t *testing.T, sut store.Store, id protocol.JobUUID) (deleted bool) {
	t.Helper()

	deleted, err := sut.DeleteJob(context.Background(), id)
	requireNoError(t, err, "job deletion should not fail")

	return deleted
}

func listEnabledJobs(t *testing.T, sut store.Store) (jobs []*store.Job) {
	t.Helper()

	jobs, err := sut.ListEnabledJobs(context.Background())
	requireNoError(t, err, "enabled job listing should not fail")

	return jobs
}

func createExecution(
	t *testing.T,
	sut store.Store,
	jobID protocol.JobUUID,
	scheduledAt time.Time,
) (created *store.Execution) {
	t.Helper()

	execution, fresh, err := sut.CreateExecution(
		context.Background(),
		jobID,
		scheduledAt,
		store.StateScheduled,
		false,
	)
	requireNoError(t, err, "execution creation should not fail")

	if !fresh {
		t.Fatal("a fresh occurrence should create an execution")
	}

	return execution
}

func getExecution(
	t *testing.T,
	sut store.Store,
	id protocol.ExecutionID,
) (fetched *store.Execution) {
	t.Helper()

	fetched, err := sut.GetExecution(context.Background(), id)
	requireNoError(t, err, "execution fetch should not fail")

	return fetched
}

func updateExecution(
	t *testing.T,
	sut store.Store,
	current *store.Execution,
	update store.ExecutionUpdate,
) (next *store.Execution) {
	t.Helper()

	next, err := sut.UpdateExecution(
		context.Background(),
		current.ID,
		current.Version,
		update,
	)
	requireNoError(t, err, "execution update should not fail")

	return next
}

func driveState(
	t *testing.T,
	sut store.Store,
	execution *store.Execution,
	states ...store.ExecutionState,
) (driven *store.Execution) {
	t.Helper()

	current := execution

	for _, state := range states {
		current = updateExecution(t, sut, current, store.ExecutionUpdate{State: &state})
	}

	return current
}

func dueExecutions(
	t *testing.T,
	sut store.Store,
	now time.Time,
	limit store.BatchLimit,
) (due []*store.Execution) {
	t.Helper()

	due, err := sut.DueExecutions(context.Background(), now, limit)
	requireNoError(t, err, "due lookup should not fail")

	return due
}

func nextWakeAt(t *testing.T, sut store.Store) (wake time.Time, found bool) {
	t.Helper()

	wake, found, err := sut.NextWakeAt(context.Background())
	requireNoError(t, err, "wake lookup should not fail")

	return wake, found
}

func executionsInStates(
	t *testing.T,
	sut store.Store,
	states ...store.ExecutionState,
) (matching []*store.Execution) {
	t.Helper()

	matching, err := sut.ExecutionsInStates(context.Background(), states...)
	requireNoError(t, err, "state filter lookup should not fail")

	return matching
}

func executionsForJob(
	t *testing.T,
	sut store.Store,
	jobID protocol.JobUUID,
) (matching []*store.Execution) {
	t.Helper()

	matching, err := sut.ExecutionsForJob(context.Background(), jobID)
	requireNoError(t, err, "job execution lookup should not fail")

	return matching
}

func hasActiveExecution(
	t *testing.T,
	sut store.Store,
	jobID protocol.JobUUID,
	exclude protocol.ExecutionID,
) (active bool) {
	t.Helper()

	active, err := sut.HasActiveExecution(context.Background(), jobID, exclude)
	requireNoError(t, err, "activity lookup should not fail")

	return active
}

func hasPendingOccurrence(
	t *testing.T,
	sut store.Store,
	jobID protocol.JobUUID,
) (pending bool) {
	t.Helper()

	pending, err := sut.HasPendingOccurrence(context.Background(), jobID)
	requireNoError(t, err, "pending lookup should not fail")

	return pending
}

func expiredLeases(
	t *testing.T,
	sut store.Store,
	now time.Time,
) (expired []*store.Execution) {
	t.Helper()

	expired, err := sut.ExpiredLeases(context.Background(), now)
	requireNoError(t, err, "expired lease lookup should not fail")

	return expired
}

func createAttempt(
	t *testing.T,
	sut store.Store,
	executionID protocol.ExecutionID,
	number store.AttemptNumber,
	instanceID protocol.InstanceID,
) (created *store.Attempt) {
	t.Helper()

	attempt, err := sut.CreateAttempt(context.Background(), executionID, number, instanceID)
	requireNoError(t, err, "attempt creation should not fail")

	return attempt
}

func getAttempt(
	t *testing.T,
	sut store.Store,
	id protocol.AttemptID,
) (fetched *store.Attempt) {
	t.Helper()

	fetched, err := sut.GetAttempt(context.Background(), id)
	requireNoError(t, err, "attempt fetch should not fail")

	return fetched
}

func updateAttemptState(
	t *testing.T,
	sut store.Store,
	id protocol.AttemptID,
	from []store.AttemptState,
	to store.AttemptState,
	errorText store.ErrorText,
) (updated bool) {
	t.Helper()

	updated, err := sut.UpdateAttemptState(context.Background(), id, from, to, errorText)
	requireNoError(t, err, "attempt update should not fail")

	return updated
}

func attemptsForExecution(
	t *testing.T,
	sut store.Store,
	executionID protocol.ExecutionID,
) (attempts []*store.Attempt) {
	t.Helper()

	attempts, err := sut.AttemptsForExecution(context.Background(), executionID)
	requireNoError(t, err, "execution attempt lookup should not fail")

	return attempts
}

func attemptsOnInstance(
	t *testing.T,
	sut store.Store,
	instanceID protocol.InstanceID,
	states ...store.AttemptState,
) (attempts []*store.Attempt) {
	t.Helper()

	attempts, err := sut.AttemptsOnInstance(context.Background(), instanceID, states...)
	requireNoError(t, err, "instance attempt lookup should not fail")

	return attempts
}

func storeResult(
	t *testing.T,
	sut store.Store,
	key store.JobKey,
	instanceID protocol.InstanceID,
) (id protocol.JobUUID, stored bool) {
	t.Helper()

	id = jobUUID(key)

	stored, err := sut.StoreResult(context.Background(), &store.PendingResult{
		JobUUID:    id,
		InstanceID: instanceID,
		Success:    true,
	})
	requireNoError(t, err, "result storage should not fail")

	return id, stored
}

func deleteResult(t *testing.T, sut store.Store, jobUUID protocol.JobUUID) (deleted bool) {
	t.Helper()

	deleted, err := sut.DeleteResult(context.Background(), jobUUID)
	requireNoError(t, err, "result deletion should not fail")

	return deleted
}

func resultsForInstance(
	t *testing.T,
	sut store.Store,
	id protocol.InstanceID,
	limit store.BatchLimit,
) (held []*store.PendingResult) {
	t.Helper()

	held, err := sut.ResultsForInstance(context.Background(), id, limit)
	requireNoError(t, err, "instance result lookup should not fail")

	return held
}

func markResultSent(
	t *testing.T,
	sut store.Store,
	jobUUID protocol.JobUUID,
	at time.Time,
) {
	t.Helper()

	err := sut.MarkResultSent(context.Background(), jobUUID, at)
	requireNoError(t, err, "marking a held result sent should not fail")
}

func expiredResults(
	t *testing.T,
	sut store.Store,
	before time.Time,
	limit store.BatchLimit,
) (expired []*store.PendingResult) {
	t.Helper()

	expired, err := sut.ExpiredResults(context.Background(), before, limit)
	requireNoError(t, err, "expired result lookup should not fail")

	return expired
}

func countResults(t *testing.T, sut store.Store) (count store.OccurrenceCount) {
	t.Helper()

	count, err := sut.CountResults(context.Background())
	requireNoError(t, err, "result count should not fail")

	return count
}
