package engine_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	metricResultsStored      = "results_stored"
	metricResultsDelivered   = "results_delivered"
	metricResultsRedelivered = "results_redelivered"
	metricResultsAcked       = "results_acked"
	metricResultsDropped     = "results_dropped"
	metricResultsExpired     = "results_expired"
	metricResultsAbandoned   = "results_abandoned"

	submitterType = protocol.ExecutorType("submitter")

	cfgResultRetention = 4 * time.Minute
	retentionStep      = 3 * time.Minute

	evictionRetention = 5 * time.Minute
	evictionStep      = 2 * time.Minute

	perInstanceResultCap = store.OccurrenceCount(2)
	doubleCount          = uint64(2)
)

var resultPayload = []byte("computed-value")

func deliverJob(key string, start time.Time) *protocol.JobUpsert {
	upsert := oneShotJob(key, start)
	upsert.ResultMode = protocol.ResultModeDeliver

	return upsert
}

func (s *fakeSender) deliveries() []*protocol.ResultDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()

	deliveries := make([]*protocol.ResultDelivery, 0, len(s.sent))

	for _, msg := range s.sent {
		if delivery, ok := msg.(*protocol.ResultDelivery); ok {
			deliveries = append(deliveries, delivery)
		}
	}

	return deliveries
}

func (s *fakeSender) deliveryCount() int {
	return len(s.deliveries())
}

func (f *engineFixture) registerSubmitter(instance protocol.InstanceID) *fakeSender {
	sender := &fakeSender{}
	f.registry.Register(instance, submitterType, unlimitedCapacity, nil, nil, sender)

	return sender
}

func (f *engineFixture) finishWithValue(
	t *testing.T,
	instance protocol.InstanceID,
	executionID protocol.ExecutionID,
	payload []byte,
) {
	t.Helper()

	execution := f.execution(t, executionID)

	f.engine.HandleExecResult(context.Background(), instance, &protocol.ExecResult{
		ExecutionID: executionID,
		AttemptID:   execution.CurrentAttemptID,
		Success:     true,
		HasValue:    true,
		Result:      payload,
	})
}

func (f *engineFixture) settleDelivered(
	t *testing.T,
	sender *fakeSender,
	upsert *protocol.JobUpsert,
	priorRequests int,
) protocol.JobUUID {
	t.Helper()

	jobID := f.upsert(t, upsert)
	f.awaitRequests(t, sender, priorRequests+1)

	executionID := f.soleExecution(t, jobID).ID
	f.accept(t, firstWorker, executionID)
	f.finishWithValue(t, firstWorker, executionID, resultPayload)
	f.awaitExecutionState(t, executionID, store.StateSucceeded)

	return jobID
}

func (f *engineFixture) countResults(t *testing.T) store.OccurrenceCount {
	t.Helper()

	count, err := f.store.CountResults(context.Background())
	if err != nil {
		t.Fatalf("result count should not fail: %v", err)
	}

	return count
}

func (f *engineFixture) heldResultIDs(
	t *testing.T,
	instance protocol.InstanceID,
) map[protocol.JobUUID]bool {
	t.Helper()

	held, err := f.store.ResultsForInstance(context.Background(), instance, 0)
	if err != nil {
		t.Fatalf("pending result listing should not fail: %v", err)
	}

	ids := make(map[protocol.JobUUID]bool, len(held))

	for _, result := range held {
		ids[result.JobUUID] = true
	}

	return ids
}

func (f *engineFixture) resultAck(
	instance protocol.InstanceID,
	jobID protocol.JobUUID,
	accepted bool,
) {
	f.engine.HandleResultAck(
		context.Background(),
		instance,
		&protocol.ResultDeliveryAck{JobUUID: jobID, Accepted: accepted},
	)
}

func (f *engineFixture) awaitDeliveries(t *testing.T, sender *fakeSender, want int) {
	t.Helper()

	await(t, "the submitter should receive the expected deliveries", func() bool {
		return sender.deliveryCount() >= want
	})
}

func TestEngineDeliversResultToSubmitter(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	submitterSender := fixture.registerSubmitter(submitterInstance)
	fixture.start(t)

	jobID := fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-deliver", baseTime.Add(-time.Minute)),
		0,
	)

	request := fixture.requestAt(t, sender, 0)
	if !request.DeliverResult {
		t.Error("a deliver-mode job should ask the executor for its result")
	}

	fixture.awaitDeliveries(t, submitterSender, 1)

	delivery := submitterSender.deliveries()[0]
	executionID := fixture.soleExecution(t, jobID).ID

	if delivery.JobUUID != jobID || delivery.ExecutionID != executionID {
		t.Errorf("the delivery should carry the settled execution: got %+v", delivery)
	}

	if !delivery.Success || !delivery.HasValue {
		t.Errorf("the delivery should carry a successful value: got %+v", delivery)
	}

	if !bytes.Equal(delivery.Result, resultPayload) {
		t.Errorf("the delivery should carry the result payload: got %q", delivery.Result)
	}

	if got := fixture.countResults(t); got != store.OccurrenceCount(singleCount) {
		t.Errorf("a delivered result must stay held until acknowledged: got %d", got)
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricResultsStored] != singleCount ||
		snapshot[metricResultsDelivered] != singleCount {
		t.Errorf("the store and delivery should be counted: got %+v", snapshot)
	}
}

func TestEngineHoldsResultAcrossDisconnect(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	fixture.settleDelivered(t, sender, deliverJob("job-hold", baseTime.Add(-time.Minute)), 0)

	if got := fixture.countResults(t); got != store.OccurrenceCount(singleCount) {
		t.Errorf("the result of an absent submitter should be held: got %d", got)
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricResultsStored] != singleCount {
		t.Errorf("the held result should be counted as stored: got %+v", snapshot)
	}

	if snapshot[metricResultsDelivered] != noCount {
		t.Errorf("nothing should be delivered while the submitter is away: got %+v", snapshot)
	}
}

func TestEngineRedeliversOnReconnect(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	fixture.settleDelivered(t, sender, deliverJob("job-reconnect", baseTime.Add(-time.Minute)), 0)

	firstSession := fixture.registerSubmitter(submitterInstance)
	fixture.engine.HandleRegistered(context.Background(), submitterInstance)
	fixture.awaitDeliveries(t, firstSession, 1)

	secondSession := fixture.registerSubmitter(submitterInstance)
	fixture.engine.HandleRegistered(context.Background(), submitterInstance)
	fixture.awaitDeliveries(t, secondSession, 1)

	if got := fixture.countResults(t); got != store.OccurrenceCount(singleCount) {
		t.Errorf("an unacknowledged result must survive redelivery: got %d", got)
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricResultsDelivered] != singleCount ||
		snapshot[metricResultsRedelivered] < singleCount {
		t.Errorf("the reconnect redelivery should be counted: got %+v", snapshot)
	}
}

func TestEngineDropsResultAfterRetention(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.ReconcileInterval = cfgReconcileFast
	cfg.ResultRetention = cfgResultRetention

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	submitterSender := fixture.registerSubmitter(submitterInstance)
	fixture.start(t)

	fixture.settleDelivered(t, sender, deliverJob("job-retention", baseTime.Add(-time.Minute)), 0)
	fixture.awaitDeliveries(t, submitterSender, 1)

	fixture.clock.Advance(retentionStep)
	fixture.awaitDeliveries(t, submitterSender, 2)

	fixture.clock.Advance(retentionStep)

	await(t, "the held result should expire once retention runs out", func() bool {
		return fixture.countResults(t) == 0
	})

	if got := fixture.engine.Snapshot()[metricResultsExpired]; got != singleCount {
		t.Errorf("the eviction should be counted: got %d", got)
	}
}

func TestEngineRejectsDeliverOnIntervalSchedule(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	upsert := intervalJob("job-deliver-interval", baseTime, time.Minute)
	upsert.ResultMode = protocol.ResultModeDeliver

	ack := fixture.engine.HandleJobUpsert(context.Background(), submitterInstance, upsert)

	if ack.Accepted {
		t.Fatal("a deliver-mode interval job must be refused")
	}

	if ack.Error == nil || !strings.Contains(ack.Error.Message, "one-shot") {
		t.Errorf("the refusal should explain the one-shot requirement: got %+v", ack.Error)
	}

	if _, err := fixture.store.GetJob(context.Background(), upsert.JobUUID); err == nil {
		t.Error("a refused upsert must not store the job")
	}
}

func TestEngineIgnoreModeStoresNothing(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	submitterSender := fixture.registerSubmitter(submitterInstance)
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-ignore", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	request := fixture.requestAt(t, sender, 0)
	if request.DeliverResult {
		t.Error("an ignore-mode job must not ask the executor for its result")
	}

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)
	fixture.finish(t, firstWorker, executionID, true, false)
	fixture.awaitExecutionState(t, executionID, store.StateSucceeded)

	if got := fixture.countResults(t); got != 0 {
		t.Errorf("ignore mode must hold nothing: got %d pending results", got)
	}

	if got := submitterSender.deliveryCount(); got != 0 {
		t.Errorf("ignore mode must deliver nothing: got %d deliveries", got)
	}

	if got := fixture.engine.Snapshot()[metricResultsStored]; got != noCount {
		t.Errorf("ignore mode must store nothing: got %d", got)
	}
}

func TestEngineEvictsOldestPerInstance(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.ReconcileInterval = cfgReconcileFast
	cfg.ResultRetention = evictionRetention

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.registerSubmitter(submitterInstance)
	fixture.start(t)

	oldest := fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-evict-oldest", baseTime.Add(-time.Minute)),
		0,
	)

	fixture.clock.Advance(evictionStep)

	middle := fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-evict-middle", baseTime.Add(-time.Minute)),
		1,
	)

	fixture.clock.Advance(evictionStep)

	newest := fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-evict-newest", baseTime.Add(-time.Minute)),
		2,
	)

	fixture.clock.Advance(evictionStep)

	await(t, "the oldest held result should be evicted first", func() bool {
		return fixture.countResults(t) == store.OccurrenceCount(doubleCount)
	})

	held := fixture.heldResultIDs(t, submitterInstance)
	if held[oldest] || !held[middle] || !held[newest] {
		t.Errorf("only the oldest result should be gone: got %v", held)
	}

	fixture.clock.Advance(evictionStep)

	await(t, "the next oldest held result should be evicted", func() bool {
		return fixture.countResults(t) == store.OccurrenceCount(singleCount)
	})

	held = fixture.heldResultIDs(t, submitterInstance)
	if held[middle] || !held[newest] {
		t.Errorf("only the newest result should remain: got %v", held)
	}

	if got := fixture.engine.Snapshot()[metricResultsExpired]; got != doubleCount {
		t.Errorf("both evictions should be counted: got %d", got)
	}
}

func TestEngineDropsResultAtPerInstanceCap(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.MaxPendingResultsPerInstance = perInstanceResultCap

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	fixture.settleDelivered(t, sender, deliverJob("job-cap-first", baseTime.Add(-time.Minute)), 0)
	fixture.settleDelivered(t, sender, deliverJob("job-cap-second", baseTime.Add(-time.Minute)), 1)
	fixture.settleDelivered(t, sender, deliverJob("job-cap-third", baseTime.Add(-time.Minute)), 2)

	if got := fixture.countResults(t); got != perInstanceResultCap {
		t.Errorf("the per-instance cap should bound held results: got %d", got)
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricResultsDropped] != singleCount {
		t.Errorf("the refused result should be counted as dropped: got %+v", snapshot)
	}

	if snapshot[metricResultsStored] != doubleCount {
		t.Errorf("only the admitted results should count as stored: got %+v", snapshot)
	}
}

func TestEngineForgedResultAckFromAnotherInstanceIsRejected(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	submitterSender := fixture.registerSubmitter(submitterInstance)
	fixture.start(t)

	jobID := fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-forged-ack", baseTime.Add(-time.Minute)),
		0,
	)
	fixture.awaitDeliveries(t, submitterSender, 1)

	fixture.resultAck(firstWorker, jobID, true)

	if got := fixture.countResults(t); got != store.OccurrenceCount(singleCount) {
		t.Fatalf("a forged ack must not delete the held result: got %d", got)
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricResultsAcked] != noCount {
		t.Errorf("a forged ack must not count as an ack: got %+v", snapshot)
	}

	if snapshot[metricStaleMessages] < singleCount {
		t.Errorf("the forged ack should be counted as stale: got %+v", snapshot)
	}

	fixture.resultAck(submitterInstance, jobID, true)

	if got := fixture.countResults(t); got != 0 {
		t.Errorf("the owning submitter's ack should still delete: got %d", got)
	}
}

func TestEngineAckDeletesPendingResult(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	submitterSender := fixture.registerSubmitter(submitterInstance)
	fixture.start(t)

	jobID := fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-ack", baseTime.Add(-time.Minute)),
		0,
	)
	fixture.awaitDeliveries(t, submitterSender, 1)

	fixture.resultAck(submitterInstance, jobID, true)

	if got := fixture.countResults(t); got != 0 {
		t.Fatalf("an accepted ack should delete the held result: got %d", got)
	}

	if got := fixture.engine.Snapshot()[metricResultsAcked]; got != singleCount {
		t.Errorf("the ack should be counted: got %d", got)
	}

	fixture.resultAck(submitterInstance, jobID, true)

	if got := fixture.engine.Snapshot()[metricResultsAcked]; got != singleCount {
		t.Errorf("an ack replay must not count twice: got %d", got)
	}
}

func TestEngineRefusedAckKeepsResult(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	submitterSender := fixture.registerSubmitter(submitterInstance)
	fixture.start(t)

	jobID := fixture.settleDelivered(
		t,
		sender,
		deliverJob("job-refused-ack", baseTime.Add(-time.Minute)),
		0,
	)
	fixture.awaitDeliveries(t, submitterSender, 1)

	fixture.resultAck(submitterInstance, jobID, false)

	if got := fixture.countResults(t); got != store.OccurrenceCount(singleCount) {
		t.Errorf("a refused delivery should stay held: got %d", got)
	}

	snapshot := fixture.engine.Snapshot()
	if snapshot[metricResultsAbandoned] != singleCount {
		t.Errorf("the refusal should be counted as abandoned: got %+v", snapshot)
	}

	if snapshot[metricResultsAcked] != noCount {
		t.Errorf("a refusal must not count as an ack: got %+v", snapshot)
	}
}
