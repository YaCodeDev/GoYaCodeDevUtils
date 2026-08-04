package engine_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	metricExecutionsExpired = "executions_expired"
	metricAttemptsPurged    = "attempts_purged"

	cfgShortBackfillMaxAge       = 90 * time.Second
	cfgExecutionRetention        = 2 * time.Minute
	executionRetentionStep       = 3 * time.Minute
	abortPollWindow              = 2 * cfgReconcileFast
	forcedDeletionFailureMessage = "forced attempt deletion failure"
)

// failingAttemptRepository wraps a store.AttemptRepository and forces
// DeleteAttempt to fail for one chosen attempt, so a test can drive the
// engine's purge-abort path deterministically.
type failingAttemptRepository struct {
	store.AttemptRepository

	failID protocol.AttemptID
}

func (f *failingAttemptRepository) DeleteAttempt(
	ctx context.Context,
	id protocol.AttemptID,
) (bool, yaerrors.Error) {
	if id == f.failID {
		return false, yaerrors.FromString(
			http.StatusInternalServerError,
			forcedDeletionFailureMessage,
		)
	}

	return f.AttemptRepository.DeleteAttempt(ctx, id)
}

func TestEngineSweepExecutionsPurgesSettledExecution(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.ReconcileInterval = cfgReconcileFast
	cfg.ExecutionRetention = cfgExecutionRetention
	cfg.BackfillMaxAge = cfgShortBackfillMaxAge

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-exec-retention", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	attemptID := fixture.execution(t, executionID).CurrentAttemptID

	fixture.finish(t, firstWorker, executionID, true, false)
	fixture.awaitExecutionState(t, executionID, store.StateSucceeded)

	fixture.clock.Advance(executionRetentionStep)

	await(t, "the settled execution should be purged once retention runs out", func() bool {
		_, err := fixture.store.GetExecution(context.Background(), executionID)

		return err != nil
	})

	if _, err := fixture.store.GetAttempt(context.Background(), attemptID); err == nil {
		t.Error("the purged execution's attempt should be gone too")
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricExecutionsExpired] != singleCount {
		t.Errorf("the purge should be counted: got %+v", snapshot)
	}

	if snapshot[metricAttemptsPurged] != singleCount {
		t.Errorf("the attempt purge should be counted: got %+v", snapshot)
	}
}

func TestEngineSweepExecutionsNeverPurgesNonTerminal(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.ReconcileInterval = cfgReconcileFast
	cfg.ExecutionRetention = cfgExecutionRetention
	cfg.BackfillMaxAge = cfgShortBackfillMaxAge

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	settledJobID := fixture.upsert(
		t,
		oneShotJob("job-exec-retention-settled", baseTime.Add(-time.Minute)),
	)
	fixture.awaitRequests(t, sender, 1)

	settledExecutionID := fixture.soleExecution(t, settledJobID).ID
	fixture.accept(t, firstWorker, settledExecutionID)
	fixture.finish(t, firstWorker, settledExecutionID, true, false)
	fixture.awaitExecutionState(t, settledExecutionID, store.StateSucceeded)

	pendingJobID := fixture.upsert(
		t,
		oneShotJob("job-exec-retention-pending", baseTime.Add(time.Hour)),
	)
	pendingExecutionID := fixture.soleExecution(t, pendingJobID).ID

	fixture.clock.Advance(executionRetentionStep)

	await(t, "the settled execution should be purged once retention runs out", func() bool {
		_, err := fixture.store.GetExecution(context.Background(), settledExecutionID)

		return err != nil
	})

	if _, err := fixture.store.GetExecution(context.Background(), pendingExecutionID); err != nil {
		t.Errorf("a non-terminal execution must never be purged: %v", err)
	}

	if got := fixture.engine.Snapshot()[metricExecutionsExpired]; got != singleCount {
		t.Errorf("only the settled execution should be counted as expired: got %d", got)
	}
}

func TestEngineSweepExecutionsAbortsPurgeOnAttemptDeletionFailure(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.ReconcileInterval = cfgReconcileFast
	cfg.ExecutionRetention = cfgExecutionRetention
	cfg.BackfillMaxAge = cfgShortBackfillMaxAge

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})

	failing := &failingAttemptRepository{AttemptRepository: fixture.store}

	custom := engine.NewEngine(
		cfg,
		fixture.store,
		fixture.store,
		failing,
		fixture.store,
		fixture.registry,
		yalogger.NewBaseLogger(nil).NewLogger(),
	)
	custom.SetClock(fixture.clock.Now)
	fixture.engine = custom

	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-exec-retention-abort", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)
	fixture.finish(t, firstWorker, executionID, true, false)
	fixture.awaitExecutionState(t, executionID, store.StateSucceeded)

	failing.failID = fixture.execution(t, executionID).CurrentAttemptID

	fixture.clock.Advance(executionRetentionStep)

	deadline := time.Now().Add(abortPollWindow)

	for time.Now().Before(deadline) {
		if _, err := fixture.store.GetExecution(context.Background(), executionID); err != nil {
			t.Fatal("a failed attempt deletion must not delete the execution")
		}

		time.Sleep(pollInterval)
	}

	if _, err := fixture.store.GetAttempt(context.Background(), failing.failID); err != nil {
		t.Errorf("the execution's attempt must survive an aborted purge: %v", err)
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricExecutionsExpired] != noCount {
		t.Errorf("an aborted purge must not count as expired: got %+v", snapshot)
	}

	if snapshot[metricAttemptsPurged] != noCount {
		t.Errorf("an aborted purge must not count any attempt as purged: got %+v", snapshot)
	}
}
