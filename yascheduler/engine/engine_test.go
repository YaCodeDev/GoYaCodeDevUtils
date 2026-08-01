package engine_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/memstore"
)

const (
	baseUnixSeconds = 1_700_000_000
	awaitTimeout    = 5 * time.Second
	pollInterval    = 5 * time.Millisecond
	stopTimeout     = 2 * time.Second

	workerType        = protocol.ExecutorType("worker")
	workerFunction    = protocol.FunctionName("report")
	otherFunction     = protocol.FunctionName("cleanup")
	firstWorker       = protocol.InstanceID("exec-1")
	secondWorker      = protocol.InstanceID("exec-2")
	submitterInstance = protocol.InstanceID("submitter-1")

	failureMessage    = "function exploded"
	rejectMessage     = "executor said no"
	unlimitedCapacity = store.Capacity(0)

	firstAttemptNumber = store.AttemptNumber(1)
	noFunctionAttempts = store.FunctionAttempts(0)
	oneFunctionAttempt = store.FunctionAttempts(1)

	metricStaleMessages     = "stale_messages"
	metricInfraRedispatches = "infra_redispatches"
	metricSkippedOverlaps   = "skipped_overlaps"
	metricDispatchFailures  = "dispatch_failures"

	cfgLease            = 30 * time.Second
	cfgShortLease       = 5 * time.Second
	cfgReconcileIdle    = time.Hour
	cfgReconcileFast    = time.Second
	cfgDispatchBatch    = store.BatchLimit(16)
	cfgRedispatchDelay  = 250 * time.Millisecond
	cfgRetryInitial     = time.Second
	cfgRetryMax         = time.Minute
	cfgBackfillMaxCount = store.OccurrenceCount(100)
	cfgBackfillMaxAge   = 24 * time.Hour
)

var (
	baseTime        = time.Unix(baseUnixSeconds, 0).UTC()
	errQueueRefused = errors.New("executor queue refused message")
)

type deliveryRefusal bool

// jobUUID derives a stable job identifier from a job key, so two upserts of
// one key in a test address one job without a random source.
func jobUUID(key string) (id protocol.JobUUID) {
	digest := sha256.Sum256([]byte(key))

	copy(id[:], digest[:len(id)])

	return id
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *fakeClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(delta)
}

type fakeSender struct {
	mu     sync.Mutex
	sent   []protocol.Message
	refuse deliveryRefusal
}

func (s *fakeSender) EnqueueMessage(msg protocol.Message) yaerrors.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.refuse {
		return yaerrors.FromError(
			http.StatusServiceUnavailable,
			errQueueRefused,
			"failed to enqueue message",
		)
	}

	s.sent = append(s.sent, msg)

	return nil
}

func (s *fakeSender) CloseConnection() {}

func (s *fakeSender) requests() []*protocol.ExecRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	requests := make([]*protocol.ExecRequest, 0, len(s.sent))

	for _, msg := range s.sent {
		if request, ok := msg.(*protocol.ExecRequest); ok {
			requests = append(requests, request)
		}
	}

	return requests
}

func (s *fakeSender) requestCount() int {
	return len(s.requests())
}

type engineFixture struct {
	engine   engine.Engine
	store    *memstore.Store
	registry engine.ExecutorRegistry
	clock    *fakeClock
}

func newFixture(t *testing.T, cfg *engine.Config) *engineFixture {
	t.Helper()

	clock := newFakeClock(baseTime)

	records := memstore.NewStore(memstore.Config{})
	records.SetClock(clock.Now)

	registry := engine.NewExecutorRegistry()

	fixture := &engineFixture{store: records, registry: registry, clock: clock}
	fixture.engine = newEngineOver(fixture, cfg)

	return fixture
}

func newEngineOver(fixture *engineFixture, cfg *engine.Config) engine.Engine {
	created := engine.NewEngine(
		cfg,
		fixture.store,
		fixture.store,
		fixture.store,
		fixture.store,
		fixture.registry,
		yalogger.NewBaseLogger(nil).NewLogger(),
	)
	created.SetClock(fixture.clock.Now)

	return created
}

func testConfig() *engine.Config {
	return &engine.Config{
		Lease:             cfgLease,
		ReconcileInterval: cfgReconcileIdle,
		DispatchBatch:     cfgDispatchBatch,
		RedispatchDelay:   cfgRedispatchDelay,
		RetryInitialDelay: cfgRetryInitial,
		RetryMaxDelay:     cfgRetryMax,
		DefaultBackfill:   protocol.BackfillModeEnabled,
		BackfillMaxCount:  cfgBackfillMaxCount,
		BackfillMaxAge:    cfgBackfillMaxAge,
	}
}

func stopEngine(running engine.Engine) {
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()

	running.Pause()
	running.Stop(ctx)
}

func (f *engineFixture) start(t *testing.T) {
	t.Helper()

	f.engine.Start(context.Background())

	t.Cleanup(func() { stopEngine(f.engine) })
}

func (f *engineFixture) registerWorker(
	instance protocol.InstanceID,
	functions ...protocol.FunctionSpec,
) (*engine.ExecutorEntry, *fakeSender) {
	return f.registerLabeledWorker(instance, nil, functions...)
}

func (f *engineFixture) registerLabeledWorker(
	instance protocol.InstanceID,
	labels []protocol.Label,
	functions ...protocol.FunctionSpec,
) (*engine.ExecutorEntry, *fakeSender) {
	sender := &fakeSender{}
	entry, _ := f.registry.Register(
		instance,
		workerType,
		unlimitedCapacity,
		functions,
		labels,
		sender,
	)

	return entry, sender
}

func (f *engineFixture) upsert(t *testing.T, upsert *protocol.JobUpsert) protocol.JobUUID {
	t.Helper()

	ack := f.engine.HandleJobUpsert(context.Background(), submitterInstance, upsert)
	if !ack.Accepted {
		t.Fatalf("job upsert should be accepted: %+v", ack.Error)
	}

	return ack.JobUUID
}

func (f *engineFixture) job(t *testing.T, jobID protocol.JobUUID) *store.Job {
	t.Helper()

	job, err := f.store.GetJob(context.Background(), jobID)
	if err != nil {
		t.Fatalf("job fetch should not fail: %v", err)
	}

	return job
}

func (f *engineFixture) executions(t *testing.T, jobID protocol.JobUUID) []*store.Execution {
	t.Helper()

	executions, err := f.store.ExecutionsForJob(context.Background(), jobID)
	if err != nil {
		t.Fatalf("execution listing should not fail: %v", err)
	}

	return executions
}

func (f *engineFixture) soleExecution(t *testing.T, jobID protocol.JobUUID) *store.Execution {
	t.Helper()

	executions := f.executions(t, jobID)
	if len(executions) != 1 {
		t.Fatalf("the job should have exactly one execution: got %d", len(executions))
	}

	return executions[0]
}

func (f *engineFixture) execution(
	t *testing.T,
	executionID protocol.ExecutionID,
) *store.Execution {
	t.Helper()

	execution, err := f.store.GetExecution(context.Background(), executionID)
	if err != nil {
		t.Fatalf("execution fetch should not fail: %v", err)
	}

	return execution
}

func (f *engineFixture) attempts(
	t *testing.T,
	executionID protocol.ExecutionID,
) []*store.Attempt {
	t.Helper()

	attempts, err := f.store.AttemptsForExecution(context.Background(), executionID)
	if err != nil {
		t.Fatalf("attempt listing should not fail: %v", err)
	}

	return attempts
}

func (f *engineFixture) attempt(t *testing.T, attemptID protocol.AttemptID) *store.Attempt {
	t.Helper()

	attempt, err := f.store.GetAttempt(context.Background(), attemptID)
	if err != nil {
		t.Fatalf("attempt fetch should not fail: %v", err)
	}

	return attempt
}

func (f *engineFixture) accept(
	t *testing.T,
	instance protocol.InstanceID,
	executionID protocol.ExecutionID,
) {
	t.Helper()

	execution := f.execution(t, executionID)

	f.engine.HandleExecAccept(context.Background(), instance, &protocol.ExecAccept{
		ExecutionID: executionID,
		AttemptID:   execution.CurrentAttemptID,
		Accepted:    true,
	})
}

func (f *engineFixture) reject(
	t *testing.T,
	instance protocol.InstanceID,
	executionID protocol.ExecutionID,
	code protocol.ErrorCode,
) {
	t.Helper()

	execution := f.execution(t, executionID)

	f.engine.HandleExecAccept(context.Background(), instance, &protocol.ExecAccept{
		ExecutionID: executionID,
		AttemptID:   execution.CurrentAttemptID,
		Accepted:    false,
		Error:       &protocol.WireError{Code: code, Message: rejectMessage},
	})
}

func (f *engineFixture) finish(
	t *testing.T,
	instance protocol.InstanceID,
	executionID protocol.ExecutionID,
	success bool,
	retryable bool,
) {
	t.Helper()

	execution := f.execution(t, executionID)

	result := &protocol.ExecResult{
		ExecutionID: executionID,
		AttemptID:   execution.CurrentAttemptID,
		Success:     success,
	}

	if !success {
		result.Error = &protocol.WireError{
			Code:      protocol.ErrorCodeFunctionError,
			Retryable: retryable,
			Message:   failureMessage,
		}
	}

	f.engine.HandleExecResult(context.Background(), instance, result)
}

func oneShotJob(key string, start time.Time) *protocol.JobUpsert {
	return &protocol.JobUpsert{
		JobUUID:      jobUUID(key),
		JobKey:       key,
		ExecutorType: workerType,
		Function:     protocol.FunctionSpec{Name: workerFunction},
		Schedule: protocol.ScheduleSpec{
			Kind:          protocol.ScheduleKindOneShot,
			StartUnixNano: start.UnixNano(),
		},
		Enabled: true,
	}
}

func intervalJob(key string, start time.Time, interval time.Duration) *protocol.JobUpsert {
	upsert := oneShotJob(key, start)
	upsert.Schedule.Kind = protocol.ScheduleKindFixedInterval
	upsert.Schedule.IntervalMillis = uint64(interval / time.Millisecond)

	return upsert
}

func await(t *testing.T, message string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(awaitTimeout)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(pollInterval)
	}

	t.Fatal(message)
}

func (f *engineFixture) awaitRequests(t *testing.T, sender *fakeSender, want int) {
	t.Helper()

	await(t, "the executor should receive the expected requests", func() bool {
		return sender.requestCount() >= want
	})
}

func (f *engineFixture) awaitExecutionState(
	t *testing.T,
	executionID protocol.ExecutionID,
	state store.ExecutionState,
) {
	t.Helper()

	await(t, "the execution should reach the expected state", func() bool {
		return f.execution(t, executionID).State == state
	})
}

func TestEngineDispatchesDueOneShot(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-dispatch", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	execution := fixture.soleExecution(t, jobID)
	request := fixture.requestAt(t, sender, 0)

	if request.JobUUID != jobID || request.ExecutionID != execution.ID {
		t.Errorf("the request should carry the dispatched execution: got %+v", request)
	}

	if request.AttemptNumber != uint32(firstAttemptNumber) {
		t.Errorf("the first dispatch should be attempt one: got %d", request.AttemptNumber)
	}

	if execution.State != store.StateDispatching {
		t.Errorf("a dispatched execution should be dispatching: got %s", execution.State)
	}

	if execution.CurrentAttemptID != request.AttemptID {
		t.Errorf(
			"the execution should be fenced to the dispatched attempt: got %d, want %d",
			execution.CurrentAttemptID,
			request.AttemptID,
		)
	}

	attempts := fixture.attempts(t, execution.ID)
	if len(attempts) != 1 {
		t.Fatalf("a single dispatch should create one attempt: got %d", len(attempts))
	}

	if attempts[0].Number != firstAttemptNumber ||
		attempts[0].State != store.AttemptDispatched ||
		attempts[0].InstanceID != firstWorker {
		t.Errorf("the attempt should be dispatched to the worker: got %+v", attempts[0])
	}
}

func (f *engineFixture) requestAt(
	t *testing.T,
	sender *fakeSender,
	index int,
) *protocol.ExecRequest {
	t.Helper()

	requests := sender.requests()
	if len(requests) <= index {
		t.Fatalf("request %d should exist: got %d requests", index, len(requests))
	}

	return requests[index]
}

func TestEngineAcceptMarksRunning(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-accept", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateRunning {
		t.Errorf("an accepted execution should be running: got %s", execution.State)
	}

	if execution.FunctionAttempts != oneFunctionAttempt {
		t.Errorf(
			"acceptance should consume one function attempt: got %d",
			execution.FunctionAttempts,
		)
	}

	attempt := fixture.attempt(t, execution.CurrentAttemptID)
	if attempt.State != store.AttemptAccepted {
		t.Errorf("the attempt should be accepted: got %d", attempt.State)
	}
}

func TestEngineResultSuccess(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-success", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	attemptID := fixture.execution(t, executionID).CurrentAttemptID
	fixture.finish(t, firstWorker, executionID, true, false)

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateSucceeded {
		t.Errorf("a successful result should settle the execution: got %s", execution.State)
	}

	if fixture.attempt(t, attemptID).State != store.AttemptSucceeded {
		t.Errorf("the attempt should be succeeded: got %d", fixture.attempt(t, attemptID).State)
	}
}

func TestEngineRetryableFailureSchedulesRetry(t *testing.T) {
	t.Parallel()

	const retryInitialMillis = uint64(60000)

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	upsert := oneShotJob("job-retry", baseTime.Add(-time.Minute))
	upsert.Retry = protocol.RetrySpec{
		Policy:             protocol.RetryPolicyFixed,
		MaxRetries:         protocol.DefaultMaxRetries,
		InitialDelayMillis: retryInitialMillis,
	}

	jobID := fixture.upsert(t, upsert)
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	attemptID := fixture.execution(t, executionID).CurrentAttemptID
	fixture.finish(t, firstWorker, executionID, false, true)

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateRetryWait {
		t.Errorf("a retryable failure should wait for a retry: got %s", execution.State)
	}

	if !execution.NextAttemptAt.After(baseTime) {
		t.Errorf("the retry should be scheduled in the future: got %v", execution.NextAttemptAt)
	}

	if execution.LastError != failureMessage {
		t.Errorf("the failure should be recorded: got %q", execution.LastError)
	}

	if fixture.attempt(t, attemptID).State != store.AttemptFunctionFailed {
		t.Errorf(
			"the attempt should be function failed: got %d",
			fixture.attempt(t, attemptID).State,
		)
	}
}

func TestEngineRetriesExhausted(t *testing.T) {
	t.Parallel()

	const (
		maxAttempts  = 4
		retryAdvance = 5 * time.Second
	)

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-exhaust", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID

	for attemptIndex := 1; attemptIndex <= maxAttempts; attemptIndex++ {
		fixture.awaitRequests(t, sender, attemptIndex)
		fixture.accept(t, firstWorker, executionID)
		fixture.finish(t, firstWorker, executionID, false, true)

		if attemptIndex < maxAttempts {
			fixture.clock.Advance(retryAdvance)
			fixture.engine.Notify()
		}
	}

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateFailed {
		t.Errorf("an exhausted execution should be failed: got %s", execution.State)
	}

	if execution.FunctionAttempts != store.FunctionAttempts(maxAttempts) {
		t.Errorf(
			"exactly %d function attempts should be consumed: got %d",
			maxAttempts,
			execution.FunctionAttempts,
		)
	}

	if execution.LastError != failureMessage {
		t.Errorf("the final failure should be recorded: got %q", execution.LastError)
	}

	attempts := fixture.attempts(t, executionID)
	if len(attempts) != maxAttempts {
		t.Fatalf("the attempt history should hold %d attempts: got %d", maxAttempts, len(attempts))
	}

	for index, attempt := range attempts {
		if attempt.State != store.AttemptFunctionFailed {
			t.Errorf("attempt %d should be function failed: got %d", index, attempt.State)
		}
	}
}

func TestEngineNonRetryableFailureFailsImmediately(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-fatal", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)
	fixture.finish(t, firstWorker, executionID, false, false)

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateFailed {
		t.Errorf("a non-retryable failure should fail immediately: got %s", execution.State)
	}

	if execution.LastError != failureMessage {
		t.Errorf("the failure should be recorded: got %q", execution.LastError)
	}

	if attempts := fixture.attempts(t, executionID); len(attempts) != 1 {
		t.Errorf("no retry should be attempted: got %d attempts", len(attempts))
	}
}

func TestEngineRejectedCapacityRedispatches(t *testing.T) {
	t.Parallel()

	const singleRedispatch = uint64(1)

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-capacity", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	attemptID := fixture.execution(t, executionID).CurrentAttemptID

	fixture.reject(t, firstWorker, executionID, protocol.ErrorCodeCapacityExhausted)

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateReady {
		t.Errorf("a capacity rejection should requeue the execution: got %s", execution.State)
	}

	if execution.FunctionAttempts != noFunctionAttempts {
		t.Errorf(
			"an infra redispatch should consume no function attempt: got %d",
			execution.FunctionAttempts,
		)
	}

	if fixture.attempt(t, attemptID).State != store.AttemptInfraFailed {
		t.Errorf(
			"the rejected attempt should be infra failed: got %d",
			fixture.attempt(t, attemptID).State,
		)
	}

	if got := fixture.engine.Snapshot()[metricInfraRedispatches]; got != singleRedispatch {
		t.Errorf("the redispatch should be counted: got %d", got)
	}
}

func TestEngineRejectedUnknownFunctionWaitsCompatible(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-unknown-fn", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.reject(t, firstWorker, executionID, protocol.ErrorCodeUnknownFunction)

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateWaitingCompatible {
		t.Errorf(
			"an unknown-function rejection should wait for compatibility: got %s",
			execution.State,
		)
	}

	if execution.WaitReason != rejectMessage {
		t.Errorf("the rejection reason should be recorded: got %q", execution.WaitReason)
	}
}

func TestEngineWaitsForExecutorThenDispatches(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-no-executor", baseTime.Add(-time.Minute)))

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.awaitExecutionState(t, executionID, store.StateWaitingExecutor)

	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})

	fixture.awaitRequests(t, sender, 1)
	fixture.awaitExecutionState(t, executionID, store.StateDispatching)
}

func TestEngineIncompatibleExecutorWaitsCompatible(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())
	fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: otherFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-incompatible", baseTime.Add(-time.Minute)))

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.awaitExecutionState(t, executionID, store.StateWaitingCompatible)
}

func driveDisconnectRedispatch(
	t *testing.T,
	fixture *engineFixture,
) (protocol.ExecutionID, protocol.AttemptID, *fakeSender) {
	t.Helper()

	firstEntry, firstSender := fixture.registerWorker(
		firstWorker,
		protocol.FunctionSpec{Name: workerFunction},
	)
	_, secondSender := fixture.registerWorker(
		secondWorker,
		protocol.FunctionSpec{Name: workerFunction},
	)

	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-disconnect", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, firstSender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	lostAttemptID := fixture.execution(t, executionID).CurrentAttemptID

	fixture.registry.Deregister(firstWorker, firstEntry.Generation())
	fixture.engine.HandleDisconnect(context.Background(), firstWorker)

	fixture.awaitRequests(t, secondSender, 1)

	return executionID, lostAttemptID, secondSender
}

func TestEngineDisconnectRedispatches(t *testing.T) {
	t.Parallel()

	const secondAttemptNumber = store.AttemptNumber(2)

	fixture := newFixture(t, testConfig())

	executionID, lostAttemptID, _ := driveDisconnectRedispatch(t, fixture)

	if fixture.attempt(t, lostAttemptID).State != store.AttemptLost {
		t.Errorf(
			"the interrupted attempt should be lost: got %d",
			fixture.attempt(t, lostAttemptID).State,
		)
	}

	execution := fixture.execution(t, executionID)
	if execution.CurrentAttemptID == lostAttemptID {
		t.Error("the redispatch should fence a fresh attempt")
	}

	if execution.FunctionAttempts != oneFunctionAttempt {
		t.Errorf(
			"an infra redispatch should consume no extra function attempt: got %d",
			execution.FunctionAttempts,
		)
	}

	attempts := fixture.attempts(t, executionID)
	if len(attempts) != 2 {
		t.Fatalf("the history should hold the lost and the fresh attempt: got %d", len(attempts))
	}

	if attempts[1].InstanceID != secondWorker || attempts[1].Number != secondAttemptNumber {
		t.Errorf("the fresh attempt should go to the second worker: got %+v", attempts[1])
	}
}

func TestEngineIgnoresLateResult(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	executionID, lostAttemptID, _ := driveDisconnectRedispatch(t, fixture)

	staleBefore := fixture.engine.Snapshot()[metricStaleMessages]
	stateBefore := fixture.execution(t, executionID).State

	fixture.engine.HandleExecResult(context.Background(), firstWorker, &protocol.ExecResult{
		ExecutionID: executionID,
		AttemptID:   lostAttemptID,
		Success:     true,
	})

	execution := fixture.execution(t, executionID)
	if execution.State != stateBefore {
		t.Errorf(
			"a late result should not move the execution: got %s, want %s",
			execution.State,
			stateBefore,
		)
	}

	if execution.State == store.StateSucceeded {
		t.Error("a late result must never complete the execution")
	}

	if got := fixture.engine.Snapshot()[metricStaleMessages]; got != staleBefore+1 {
		t.Errorf("a late result should count as stale: got %d, want %d", got, staleBefore+1)
	}
}

func TestEngineIgnoresSupersededAttemptResult(t *testing.T) {
	t.Parallel()

	const leaseAdvance = 10 * time.Second

	cfg := testConfig()
	cfg.Lease = cfgShortLease
	cfg.ReconcileInterval = cfgReconcileFast

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-superseded", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	fixture.accept(t, firstWorker, executionID)

	supersededAttemptID := fixture.execution(t, executionID).CurrentAttemptID

	fixture.clock.Advance(leaseAdvance)

	await(t, "the expired lease should be reaped and redispatched", func() bool {
		return fixture.attempt(t, supersededAttemptID).State == store.AttemptLost &&
			sender.requestCount() >= 2
	})

	fixture.awaitExecutionState(t, executionID, store.StateDispatching)
	fixture.accept(t, firstWorker, executionID)
	fixture.awaitExecutionState(t, executionID, store.StateRunning)

	currentAttemptID := fixture.execution(t, executionID).CurrentAttemptID
	if currentAttemptID == supersededAttemptID {
		t.Fatal("the redispatch should fence a fresh attempt")
	}

	staleBefore := fixture.engine.Snapshot()[metricStaleMessages]

	fixture.engine.HandleExecResult(context.Background(), firstWorker, &protocol.ExecResult{
		ExecutionID: executionID,
		AttemptID:   supersededAttemptID,
		Success:     true,
	})

	execution := fixture.execution(t, executionID)
	if execution.State != store.StateRunning {
		t.Errorf(
			"a result for a superseded attempt must not settle the execution: got %s",
			execution.State,
		)
	}

	if execution.CurrentAttemptID != currentAttemptID {
		t.Errorf(
			"the fenced attempt should be untouched: got %d, want %d",
			execution.CurrentAttemptID,
			currentAttemptID,
		)
	}

	if fixture.attempt(t, currentAttemptID).State != store.AttemptAccepted {
		t.Errorf(
			"the live attempt should stay accepted: got %d",
			fixture.attempt(t, currentAttemptID).State,
		)
	}

	if got := fixture.engine.Snapshot()[metricStaleMessages]; got != staleBefore+1 {
		t.Errorf(
			"a superseded result should count as stale: got %d, want %d",
			got,
			staleBefore+1,
		)
	}
}

func TestEngineBackfillEnabled(t *testing.T) {
	t.Parallel()

	const (
		startOffset     = 9*time.Minute + 30*time.Second
		backfillCap     = uint32(3)
		expectedTotal   = 4
		expectedSkipped = store.OccurrenceCount(7)
	)

	fixture := newFixture(t, testConfig())

	upsert := intervalJob("job-backfill", baseTime.Add(-startOffset), time.Minute)
	upsert.Backfill = protocol.BackfillSpec{
		Mode:     protocol.BackfillModeEnabled,
		MaxCount: backfillCap,
	}

	jobID := fixture.upsert(t, upsert)

	executions := fixture.executions(t, jobID)
	if len(executions) != expectedTotal {
		t.Fatalf(
			"backfill should create %d executions: got %d",
			expectedTotal,
			len(executions),
		)
	}

	backfilled := 0

	for _, execution := range executions {
		if execution.Backfilled {
			backfilled++
		}
	}

	if store.OccurrenceCount(backfilled) != store.OccurrenceCount(backfillCap) {
		t.Errorf("exactly %d executions should be backfilled: got %d", backfillCap, backfilled)
	}

	future := executions[len(executions)-1]
	if bool(future.Backfilled) || !future.ScheduledAt.After(baseTime) {
		t.Errorf("the last execution should be the future occurrence: got %+v", future)
	}

	if got := fixture.job(t, jobID).SkippedOccurrences; got != expectedSkipped {
		t.Errorf(
			"the job should record %d skipped occurrences: got %d",
			expectedSkipped,
			got,
		)
	}
}

func TestEngineBackfillDisabled(t *testing.T) {
	t.Parallel()

	const (
		startOffset     = 9*time.Minute + 30*time.Second
		expectedSkipped = store.OccurrenceCount(10)
	)

	fixture := newFixture(t, testConfig())

	upsert := intervalJob("job-no-backfill", baseTime.Add(-startOffset), time.Minute)
	upsert.Backfill = protocol.BackfillSpec{Mode: protocol.BackfillModeDisabled}

	jobID := fixture.upsert(t, upsert)

	executions := fixture.executions(t, jobID)
	if len(executions) != 1 {
		t.Fatalf("only the future occurrence should exist: got %d", len(executions))
	}

	if bool(executions[0].Backfilled) || !executions[0].ScheduledAt.After(baseTime) {
		t.Errorf("the only execution should be the future occurrence: got %+v", executions[0])
	}

	if got := fixture.job(t, jobID).SkippedOccurrences; got != expectedSkipped {
		t.Errorf(
			"every missed occurrence should count as skipped: got %d, want %d",
			got,
			expectedSkipped,
		)
	}
}

func TestEngineOverlapSkip(t *testing.T) {
	t.Parallel()

	const (
		startOffset  = 30 * time.Second
		singleSkip   = uint64(1)
		overlapAhead = time.Minute
	)

	fixture := newFixture(t, testConfig())
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	upsert := intervalJob("job-overlap", baseTime.Add(-startOffset), time.Minute)
	upsert.Overlap = protocol.OverlapPolicySkip

	jobID := fixture.upsert(t, upsert)
	fixture.awaitRequests(t, sender, 1)

	first := fixture.requestAt(t, sender, 0)
	fixture.accept(t, firstWorker, first.ExecutionID)

	var secondID protocol.ExecutionID

	for _, execution := range fixture.executions(t, jobID) {
		if execution.ScheduledAt.After(baseTime) {
			secondID = execution.ID

			break
		}
	}

	if secondID == 0 {
		t.Fatal("the future occurrence should already be materialized")
	}

	fixture.clock.Advance(overlapAhead)
	fixture.engine.Notify()

	fixture.awaitExecutionState(t, secondID, store.StateSkipped)

	if got := fixture.engine.Snapshot()[metricSkippedOverlaps]; got != singleSkip {
		t.Errorf("the overlap skip should be counted: got %d", got)
	}
}

func TestEngineDisableCancelsPending(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	const jobKey = "job-disable"

	jobID := fixture.upsert(t, oneShotJob(jobKey, baseTime.Add(time.Hour)))

	executionID := fixture.soleExecution(t, jobID).ID
	if got := fixture.execution(t, executionID).State; got != store.StateScheduled {
		t.Fatalf("the pending execution should be scheduled: got %s", got)
	}

	disabled := oneShotJob(jobKey, baseTime.Add(time.Hour))
	disabled.Enabled = false

	fixture.upsert(t, disabled)

	if got := fixture.execution(t, executionID).State; got != store.StateCancelled {
		t.Errorf("disabling the job should cancel pending executions: got %s", got)
	}
}

func TestEngineLeaseExpiryRequeues(t *testing.T) {
	t.Parallel()

	const leaseAdvance = 10 * time.Second

	cfg := testConfig()
	cfg.Lease = cfgShortLease
	cfg.ReconcileInterval = cfgReconcileFast

	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-lease", baseTime.Add(-time.Minute)))
	fixture.awaitRequests(t, sender, 1)

	executionID := fixture.soleExecution(t, jobID).ID
	lostAttemptID := fixture.execution(t, executionID).CurrentAttemptID

	fixture.clock.Advance(leaseAdvance)

	await(t, "the expired lease should be reaped and redispatched", func() bool {
		return fixture.attempt(t, lostAttemptID).State == store.AttemptLost &&
			sender.requestCount() >= 2
	})

	if got := fixture.execution(t, executionID).FunctionAttempts; got != noFunctionAttempts {
		t.Errorf("a lease reap should consume no function attempt: got %d", got)
	}
}

func TestEngineRestartRecovery(t *testing.T) {
	t.Parallel()

	const totalRequests = 4

	cfg := testConfig()
	fixture := newFixture(t, cfg)
	_, sender := fixture.registerWorker(firstWorker, protocol.FunctionSpec{Name: workerFunction})
	fixture.start(t)

	dispatchedJobID := fixture.upsert(
		t,
		oneShotJob("job-restart-dispatched", baseTime.Add(-2*time.Minute)),
	)
	fixture.awaitRequests(t, sender, 1)

	dispatchedID := fixture.soleExecution(t, dispatchedJobID).ID
	dispatchedAttemptID := fixture.execution(t, dispatchedID).CurrentAttemptID

	runningJobID := fixture.upsert(
		t,
		oneShotJob("job-restart-running", baseTime.Add(-time.Minute)),
	)
	fixture.awaitRequests(t, sender, 2)

	runningID := fixture.soleExecution(t, runningJobID).ID
	fixture.accept(t, firstWorker, runningID)

	runningAttemptID := fixture.execution(t, runningID).CurrentAttemptID

	stopEngine(fixture.engine)

	secondEngine := newEngineOver(fixture, cfg)
	secondEngine.Start(context.Background())

	t.Cleanup(func() { stopEngine(secondEngine) })

	await(t, "restart recovery should redispatch interrupted executions", func() bool {
		return sender.requestCount() >= totalRequests
	})

	if fixture.attempt(t, dispatchedAttemptID).State != store.AttemptLost {
		t.Errorf(
			"the interrupted dispatch should be lost: got %d",
			fixture.attempt(t, dispatchedAttemptID).State,
		)
	}

	if fixture.attempt(t, runningAttemptID).State != store.AttemptLost {
		t.Errorf(
			"the interrupted run should be lost: got %d",
			fixture.attempt(t, runningAttemptID).State,
		)
	}

	if got := fixture.execution(t, dispatchedID).FunctionAttempts; got != noFunctionAttempts {
		t.Errorf("recovering an unaccepted dispatch should keep zero attempts: got %d", got)
	}

	if got := fixture.execution(t, runningID).FunctionAttempts; got != oneFunctionAttempt {
		t.Errorf("recovering an accepted run should keep its consumed attempt: got %d", got)
	}
}

func TestEngineEnqueueFailureRequeues(t *testing.T) {
	t.Parallel()

	fixture := newFixture(t, testConfig())

	sender := &fakeSender{refuse: true}
	fixture.registry.Register(
		firstWorker,
		workerType,
		unlimitedCapacity,
		[]protocol.FunctionSpec{{Name: workerFunction}},
		nil,
		sender,
	)

	fixture.start(t)

	jobID := fixture.upsert(t, oneShotJob("job-refused", baseTime.Add(-time.Minute)))

	executionID := fixture.soleExecution(t, jobID).ID

	await(t, "a refused enqueue should requeue the execution", func() bool {
		return fixture.engine.Snapshot()[metricDispatchFailures] >= 1 &&
			fixture.execution(t, executionID).State == store.StateReady
	})

	attempts := fixture.attempts(t, executionID)
	if len(attempts) == 0 {
		t.Fatal("the refused dispatch should leave an attempt behind")
	}

	if attempts[0].State != store.AttemptInfraFailed {
		t.Errorf("the refused attempt should be infra failed: got %d", attempts[0].State)
	}

	if got := fixture.execution(t, executionID).FunctionAttempts; got != noFunctionAttempts {
		t.Errorf("a refused enqueue should consume no function attempt: got %d", got)
	}
}
