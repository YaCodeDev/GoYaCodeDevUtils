package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	deletePinLabel protocol.Label = "delete-shard-1"

	deletedReason = store.WaitReason("job deleted")

	singleResult = store.OccurrenceCount(1)
	noResults    = store.OccurrenceCount(0)
)

func (f *engineFixture) deleteJob(key string) *protocol.JobDeleteAck {
	return f.engine.HandleJobDelete(
		context.Background(),
		submitterInstance,
		&protocol.JobDelete{JobKey: key, ExecutorType: workerType},
	)
}

func TestEngineJobDeleteCancelsPendingExecution(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-delete-pending", baseTime.Add(time.Hour)))
	executionID := fixture.soleExecution(t, jobID).ID

	ack := fixture.deleteJob("job-delete-pending")
	if ack.Error != nil {
		t.Fatalf("the delete should be accepted: %+v", ack.Error)
	}

	if !ack.Deleted {
		t.Fatal("deleting a stored job should report true")
	}

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateCancelled {
		t.Errorf("the pending execution should be cancelled: got %s", execution.State)
	}

	if execution.WaitReason != deletedReason {
		t.Errorf(
			"the cancellation should carry the delete reason: got %q",
			execution.WaitReason,
		)
	}
}

func TestEngineJobDeleteCancelsWaitingLabelExecution(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, pinnedJob(
		"job-delete-waiting-label",
		baseTime.Add(-time.Minute),
		deletePinLabel,
		protocol.PinPolicyStrict,
	))

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.awaitExecutionState(t, executionID, store.StateWaitingLabel)

	ack := fixture.deleteJob("job-delete-waiting-label")
	if !ack.Deleted || ack.Error != nil {
		t.Fatalf("the delete should be accepted: %+v", ack)
	}

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateCancelled {
		t.Errorf("the parked execution should be cancelled: got %s", execution.State)
	}

	if execution.WaitReason != deletedReason {
		t.Errorf(
			"the cancellation should carry the delete reason: got %q",
			execution.WaitReason,
		)
	}
}

func TestEngineJobDeleteDropsHeldResult(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(
		firstWorker,
		protocol.FunctionSpec{Name: workerFunction},
	)
	fixture.start(t)

	fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-delete-held", baseTime.Add(-time.Minute)),
		0,
	)

	if got := fixture.countResults(t); got != singleResult {
		t.Fatalf("the settled result should be held: got %d", got)
	}

	ack := fixture.deleteJob("job-delete-held")
	if !ack.Deleted || ack.Error != nil {
		t.Fatalf("the delete should be accepted: %+v", ack)
	}

	if got := fixture.countResults(t); got != noResults {
		t.Errorf("the held result should be gone after the delete: got %d", got)
	}
}

func TestEngineJobDeleteUnknownKeyAnswersFalse(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	ack := fixture.deleteJob("job-delete-missing")
	if ack.Error != nil {
		t.Fatalf("deleting an absent job should not error: %+v", ack.Error)
	}

	if ack.Deleted {
		t.Error("deleting an absent job should report false")
	}
}

func TestEngineJobDeleteRefusesMalformed(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	missingKey := fixture.engine.HandleJobDelete(
		context.Background(),
		submitterInstance,
		&protocol.JobDelete{ExecutorType: workerType},
	)
	if missingKey.Error == nil ||
		missingKey.Error.Code != protocol.ErrorCodeMalformedFrame {
		t.Errorf("an empty key should be refused as malformed: %+v", missingKey.Error)
	}

	if missingKey.Deleted {
		t.Error("a refused delete should not report deleted")
	}

	missingType := fixture.engine.HandleJobDelete(
		context.Background(),
		submitterInstance,
		&protocol.JobDelete{JobKey: "job-delete-typeless"},
	)
	if missingType.Error == nil ||
		missingType.Error.Code != protocol.ErrorCodeMalformedFrame {
		t.Errorf(
			"an empty executor type should be refused as malformed: %+v",
			missingType.Error,
		)
	}
}

func TestEngineJobDeleteFreesKeyForFreshJob(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	fixture.start(t)

	firstID := fixture.upsert(t, oneShotJob("job-delete-fresh", baseTime.Add(time.Hour)))
	firstExecutionID := fixture.soleExecution(t, firstID).ID

	ack := fixture.deleteJob("job-delete-fresh")
	if !ack.Deleted || ack.Error != nil {
		t.Fatalf("the delete should be accepted: %+v", ack)
	}

	replacement := oneShotJob("job-delete-fresh", baseTime.Add(2*time.Hour))
	replacement.JobUUID = jobUUID("job-delete-fresh-second")

	secondID := fixture.upsert(t, replacement)
	if secondID != replacement.JobUUID {
		t.Fatalf(
			"the re-upsert should keep the fresh identity: got %s, want %s",
			secondID,
			replacement.JobUUID,
		)
	}

	if got := fixture.job(t, secondID).Version; got != 1 {
		t.Errorf("the re-upsert should materialize at version 1: got %d", got)
	}

	if got := fixture.execution(t, firstExecutionID).State; got != store.StateCancelled {
		t.Errorf("the old job's execution should stay cancelled: got %s", got)
	}

	if executions := fixture.executions(t, secondID); len(executions) != 1 {
		t.Errorf("the fresh job should own one pending execution: got %d", len(executions))
	}
}

func TestEngineJobDeleteLeavesRunningToSettle(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(
		firstWorker,
		protocol.FunctionSpec{Name: workerFunction},
	)
	fixture.start(t)

	jobID := fixture.upsert(t, deliverJob("job-delete-running", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	ack := fixture.deleteJob("job-delete-running")
	if !ack.Deleted || ack.Error != nil {
		t.Fatalf("the delete should be accepted: %+v", ack)
	}

	if got := fixture.execution(t, executionID).State; got != store.StateRunning {
		t.Fatalf("a running execution should be left to finish: got %s", got)
	}

	fixture.finishWithValue(t, firstWorker, executionID, resultPayload)

	if got := fixture.execution(t, executionID).State; got != store.StateSucceeded {
		t.Errorf("the late settle should succeed the execution: got %s", got)
	}

	if got := fixture.countResults(t); got != noResults {
		t.Errorf("a late result of a deleted job should not be held: got %d", got)
	}
}

func TestEngineJobDeleteFailureSettlesWithoutJob(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(
		firstWorker,
		protocol.FunctionSpec{Name: workerFunction},
	)
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-delete-late-failure", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	ack := fixture.deleteJob("job-delete-late-failure")
	if !ack.Deleted || ack.Error != nil {
		t.Fatalf("the delete should be accepted: %+v", ack)
	}

	fixture.finish(t, firstWorker, executionID, false, true)

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateFailed {
		t.Errorf(
			"a retryable failure of a deleted job should settle failed: got %s",
			execution.State,
		)
	}

	if execution.LastError != store.ErrorText(failureMessage) {
		t.Errorf("the failure text should be recorded: got %q", execution.LastError)
	}
}
