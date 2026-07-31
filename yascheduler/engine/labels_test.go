package engine_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	metricWaitingLabel         = "waiting_label"
	metricWaitingCapacity      = "waiting_capacity"
	metricLabelPinFallbacks    = "label_pin_fallbacks"
	metricLabelUpdatesRejected = "label_updates_rejected"
	metricLabelWithdrawn       = "label_withdrawn_in_flight"

	occupiedSlot = protocol.AttemptID(9001)

	singleCount = uint64(1)
	noCount     = uint64(0)
)

func pinnedJob(
	key string,
	start time.Time,
	label protocol.Label,
	policy protocol.PinPolicy,
) *protocol.JobUpsert {
	upsert := oneShotJob(key, start)
	upsert.Pin = protocol.PinSpec{Label: label, Policy: policy}

	return upsert
}

func (f *engineFixture) labelUpdate(
	instance protocol.InstanceID,
	announce []protocol.Label,
	withdraw []protocol.Label,
) *protocol.LabelUpdateAck {
	return f.engine.HandleLabelUpdate(
		context.Background(),
		instance,
		&protocol.LabelUpdate{Announce: announce, Withdraw: withdraw},
	)
}

func TestEngineStrictPinParksWithoutLabel(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, pinnedJob(
		"job-strict-pin",
		baseTime.Add(-time.Minute),
		gpuLabel,
		protocol.PinPolicyStrict,
	))

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.awaitExecutionState(t, executionID, store.StateWaitingLabel)

	if got := sender.requestCount(); got != 0 {
		t.Errorf("a strict pin must never dispatch to an unlabeled executor: got %d", got)
	}

	if got := fixture.execution(t, executionID).WaitReason; got == "" {
		t.Error("the label park should record why the execution is held back")
	}

	if got := fixture.engine.Snapshot()[metricWaitingLabel]; got < singleCount {
		t.Errorf("the label park should be counted: got %d", got)
	}
}

func TestEngineStrictPinDispatchesToLabelHolder(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	_, plainSender := fixture.registerWorker(
		firstWorker,
		protocol.FunctionSpec{Name: workerFunction},
	)
	_, labeledSender := fixture.registerLabeledWorker(
		secondWorker,
		[]protocol.Label{gpuLabel},
		protocol.FunctionSpec{Name: workerFunction},
	)

	fixture.start(t)

	jobID := fixture.upsert(t, pinnedJob(
		"job-pin-holder",
		baseTime.Add(-time.Minute),
		gpuLabel,
		protocol.PinPolicyStrict,
	))

	fixture.awaitRequests(t, labeledSender, 1)

	if got := plainSender.requestCount(); got != 0 {
		t.Errorf("the unlabeled executor should get nothing: got %d requests", got)
	}

	executionID := fixture.soleExecution(t, jobID).ID

	attempts := fixture.attempts(t, executionID)
	if len(attempts) != 1 {
		t.Fatalf("a pinned dispatch should create one attempt: got %d", len(attempts))
	}

	if attempts[0].InstanceID != secondWorker {
		t.Errorf("the attempt should go to the label holder: got %s", attempts[0].InstanceID)
	}

	if got := fixture.engine.Snapshot()[metricLabelPinFallbacks]; got != noCount {
		t.Errorf("a satisfied pin should never count as a fallback: got %d", got)
	}
}

func TestEnginePreferredPinFallsBackToAnyExecutor(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, pinnedJob(
		"job-preferred-pin",
		baseTime.Add(-time.Minute),
		gpuLabel,
		protocol.PinPolicyPreferred,
	))

	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.awaitExecutionState(t, executionID, store.StateDispatching)

	if got := fixture.engine.Snapshot()[metricLabelPinFallbacks]; got != singleCount {
		t.Errorf("the widened pin should be counted once: got %d", got)
	}

	if got := fixture.engine.Snapshot()[metricWaitingLabel]; got != noCount {
		t.Errorf("a preferred pin should never park for its label: got %d", got)
	}
}

func TestEngineLabelAnnounceWakesParkedExecution(t *testing.T) {
	t.Parallel()

	const activeAfterAnnounce = uint32(1)

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, pinnedJob(
		"job-pin-announce",
		baseTime.Add(-time.Minute),
		gpuLabel,
		protocol.PinPolicyStrict,
	))

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.awaitExecutionState(t, executionID, store.StateWaitingLabel)

	ack := fixture.labelUpdate(firstWorker, []protocol.Label{gpuLabel}, nil)
	if !ack.Accepted {
		t.Fatalf("a label announcement within the cap should be admitted: %+v", ack.Error)
	}

	if ack.ActiveCount != activeAfterAnnounce {
		t.Errorf(
			"the ack should report the label the connection now holds: got %d",
			ack.ActiveCount,
		)
	}

	fixture.awaitRequests(t, sender, 1)
	fixture.awaitExecutionState(t, executionID, store.StateDispatching)
}

func TestEngineLabelWithdrawLeavesRunningJobAlone(t *testing.T) {
	t.Parallel()

	const noActiveLabels = uint32(0)

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerLabeledWorker(
		firstWorker,
		[]protocol.Label{gpuLabel},
		protocol.FunctionSpec{Name: workerFunction},
	)
	fixture.start(t)

	jobID := fixture.upsert(t, pinnedJob(
		"job-pin-withdraw",
		baseTime.Add(-time.Minute),
		gpuLabel,
		protocol.PinPolicyStrict,
	))

	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	attemptID := fixture.execution(t, executionID).CurrentAttemptID

	ack := fixture.labelUpdate(firstWorker, nil, []protocol.Label{gpuLabel})
	if !ack.Accepted {
		t.Fatalf("a withdrawal should be admitted: %+v", ack.Error)
	}

	if ack.ActiveCount != noActiveLabels {
		t.Errorf("the connection should hold no label afterwards: got %d", ack.ActiveCount)
	}

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateRunning {
		t.Errorf("a withdrawal must not disturb a running attempt: got %s", execution.State)
	}

	if execution.CurrentAttemptID != attemptID {
		t.Errorf("the running attempt should stay fenced: got %d", execution.CurrentAttemptID)
	}

	if got := fixture.attempt(t, attemptID).State; got != store.AttemptAccepted {
		t.Errorf("the running attempt should stay accepted: got %d", got)
	}

	if got := fixture.engine.Snapshot()[metricLabelWithdrawn]; got != singleCount {
		t.Errorf("a withdrawal under load should be counted once: got %d", got)
	}

	fixture.finish(t, firstWorker, executionID, true, false)

	if got := fixture.execution(t, executionID).State; got != store.StateSucceeded {
		t.Errorf("the attempt should still be allowed to settle: got %s", got)
	}
}

// TestEngineStrictPinBusyHolderRequeuesForCapacity keeps the two routing
// cases apart: a label nobody announces is a routing failure and parks in
// StateWaitingLabel, while a label whose only holder is saturated is a
// timing problem and takes the existing capacity requeue. Conflating them
// would make a momentarily busy instance look like a missing one.
func TestEngineStrictPinBusyHolderRequeuesForCapacity(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	sender := &fakeSender{}

	entry, _ := fixture.registry.Register(
		firstWorker,
		workerType,
		singleSlot,
		[]protocol.FunctionSpec{{Name: workerFunction}},
		[]protocol.Label{gpuLabel},
		sender,
	)
	entry.AddInFlight(occupiedSlot)

	fixture.start(t)

	jobID := fixture.upsert(t, pinnedJob(
		"job-pin-busy",
		baseTime.Add(-time.Minute),
		gpuLabel,
		protocol.PinPolicyStrict,
	))

	executionID := fixture.soleExecution(t, jobID).ID

	await(t, "a saturated label holder should requeue for capacity", func() bool {
		return fixture.engine.Snapshot()[metricWaitingCapacity] >= singleCount
	})

	if got := fixture.execution(t, executionID).State; got == store.StateWaitingLabel {
		t.Errorf("an advertised label with a busy holder is not a routing failure: got %s", got)
	}

	if got := fixture.engine.Snapshot()[metricWaitingLabel]; got != noCount {
		t.Errorf("a busy label holder should never count as a label park: got %d", got)
	}

	if got := sender.requestCount(); got != 0 {
		t.Errorf("a saturated executor should receive nothing: got %d", got)
	}
}

func TestEngineUnpinnedJobIgnoresLabels(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerLabeledWorker(
		firstWorker,
		[]protocol.Label{gpuLabel, shardLabel},
		protocol.FunctionSpec{Name: workerFunction},
	)
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-unpinned", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.awaitExecutionState(t, executionID, store.StateDispatching)

	if got := fixture.job(t, jobID).Pin.Label; got != "" {
		t.Errorf("an unpinned job should carry no label: got %q", got)
	}

	if got := fixture.engine.Snapshot()[metricWaitingLabel]; got != noCount {
		t.Errorf("an unpinned job should never park for a label: got %d", got)
	}

	if got := fixture.engine.Snapshot()[metricLabelPinFallbacks]; got != noCount {
		t.Errorf("an unpinned job should never widen a pin: got %d", got)
	}

	if got := fixture.registry.LabelPoolSize(gpuLabel); got != store.PoolSize(1) {
		t.Errorf("an unpinned dispatch should leave the label rings alone: got %d", got)
	}
}

func TestHandleLabelUpdateRejectsOverLimit(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the announcement exceeds the cap / then the ack carries the refusal",
		func(t *testing.T) {
			t.Parallel()

			fixture := newFixture(t, testConfig())
			fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})

			over := make([]protocol.Label, 0, int(protocol.DefaultMaxLabels)+1)
			for index := range int(protocol.DefaultMaxLabels) + 1 {
				over = append(over, protocol.Label("label-"+strconv.Itoa(index)))
			}

			ack := fixture.labelUpdate(firstWorker, over, nil)
			if ack.Accepted {
				t.Fatal("an announcement past the cap should be refused")
			}

			if ack.Error == nil {
				t.Fatal("a refusal must state its reason on the wire")
			}

			if ack.Error.Code != protocol.ErrorCodeLabelRejected {
				t.Errorf("the refusal should be a label rejection: got %d", ack.Error.Code)
			}

			if ack.ActiveCount != 0 {
				t.Errorf("a refused update should leave no label active: got %d", ack.ActiveCount)
			}

			if got := fixture.registry.LabelPoolSize(over[0]); got != 0 {
				t.Errorf("a refused update should apply nothing: got %d", got)
			}

			if got := fixture.engine.Snapshot()[metricLabelUpdatesRejected]; got != singleCount {
				t.Errorf("the refusal should be counted once: got %d", got)
			}
		},
	)

	t.Run(
		"when an empty label is announced / then the ack carries the refusal",
		func(t *testing.T) {
			t.Parallel()

			fixture := newFixture(t, testConfig())
			fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})

			ack := fixture.labelUpdate(firstWorker, []protocol.Label{gpuLabel, ""}, nil)
			if ack.Accepted {
				t.Fatal("an empty label should be refused")
			}

			if ack.Error == nil || ack.Error.Code != protocol.ErrorCodeLabelRejected {
				t.Fatalf("a refusal should state a label rejection: got %+v", ack.Error)
			}

			if got := fixture.registry.LabelPoolSize(gpuLabel); got != 0 {
				t.Errorf("a refused update should apply nothing: got %d", got)
			}
		},
	)
}
