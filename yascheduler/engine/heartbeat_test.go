package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	heartbeatStep     = 2 * time.Second
	heartbeatCycles   = 6
	renewalHoldWindow = 2 * cfgReconcileFast
)

var settledSeedJobKeys = []string{
	"job-heartbeat-settled-first",
	"job-heartbeat-settled-second",
	"job-heartbeat-settled-third",
}

func TestEngineHeartbeatRenewsLeaseDespiteSettledHistory(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Lease = cfgShortLease
	cfg.ReconcileInterval = cfgReconcileFast

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	for index, key := range settledSeedJobKeys {
		jobID := fixture.upsert(t, oneShotJob(key, baseTime.Add(-time.Minute)))
		fixture.awaitRequests(t, sender, index+1)

		settledID := fixture.soleExecution(t, jobID).ID
		fixture.accept(t, firstWorker, settledID)
		fixture.finish(t, firstWorker, settledID, true, false)
		fixture.awaitExecutionState(t, settledID, store.StateSucceeded)
	}

	jobID := fixture.upsert(t, oneShotJob("job-heartbeat-long-run", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, len(settledSeedJobKeys)+1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)
	fixture.awaitExecutionState(t, executionID, store.StateRunning)

	attemptID := fixture.execution(t, executionID).CurrentAttemptID

	held, err := fixture.store.AttemptsOnInstance(context.Background(), firstWorker)
	if err != nil {
		t.Fatalf("instance attempt lookup should not fail: %v", err)
	}

	if len(held) != 1 || held[0].ID != attemptID {
		t.Fatalf("settled attempts should leave the instance listing: got %v", held)
	}

	leaseBefore := fixture.execution(t, executionID).LeaseExpiresAt

	for cycle := 0; cycle < heartbeatCycles; cycle++ {
		fixture.clock.Advance(heartbeatStep)
		fixture.engine.HandleHeartbeat(context.Background(), firstWorker)
	}

	if got := fixture.execution(t, executionID).LeaseExpiresAt; !got.After(leaseBefore) {
		t.Errorf(
			"heartbeats should keep renewing the lease: got %v, want after %v",
			got,
			leaseBefore,
		)
	}

	deadline := time.Now().Add(renewalHoldWindow)

	for time.Now().Before(deadline) {
		if got := fixture.execution(t, executionID).State; got != store.StateRunning {
			t.Fatalf("a renewed lease must keep the execution running: got %s", got)
		}

		time.Sleep(pollInterval)
	}

	if got := fixture.execution(t, executionID).CurrentAttemptID; got != attemptID {
		t.Errorf("the running attempt should stay fenced: got %d, want %d", got, attemptID)
	}

	attempts := fixture.attempts(t, executionID)
	if len(attempts) != 1 {
		t.Fatalf("the execution should be dispatched exactly once: got %d attempts", len(attempts))
	}

	if attempts[0].State != store.AttemptAccepted {
		t.Errorf("the live attempt should stay accepted: got %d", attempts[0].State)
	}
}
