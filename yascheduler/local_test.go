package yascheduler_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/memstore"
)

const (
	localExecutorType     protocol.ExecutorType = "local-executor"
	localFunctionName     protocol.FunctionName = "local-report"
	localVoidFunctionName protocol.FunctionName = "local-void"
	localPinnedLabel      protocol.Label        = "local-shard-7"

	localArgValue = int64(21)

	localAwaitTimeout   = 5 * time.Second
	localExecuteTimeout = 10 * time.Second
	localStopTimeout    = 10 * time.Second
	localPollInterval   = 5 * time.Millisecond

	localReconcileFast  = 25 * time.Millisecond
	localRedispatchFast = 10 * time.Millisecond

	localDrainSleep   = 300 * time.Millisecond
	localDrainTimeout = 5 * time.Second

	localQueueFullJobs     = 12
	localQueueFullCapacity = 8
	localQueueFullBuffer   = 64

	// localQueueFullLease bounds both the lease and the renewal age in the
	// queue-full test, so an ExecAccept dropped by the size-one engine-bound
	// queue is reaped and redispatched within the test window instead of
	// being renewed by the heartbeat pump for the default hour.
	localQueueFullLease = 400 * time.Millisecond

	localHeartbeatLease = 400 * time.Millisecond
	localLongRuntime    = 2 * time.Second
	localSettleGrace    = 500 * time.Millisecond

	localPumpLease     = 150 * time.Millisecond
	localReconcileIdle = time.Hour
)

func fastLocalEngine() engine.Config {
	return engine.Config{
		ReconcileInterval: localReconcileFast,
		RedispatchDelay:   localRedispatchFast,
	}
}

func oneShotNow() protocol.ScheduleSpec {
	return protocol.ScheduleSpec{
		Kind:          protocol.ScheduleKindOneShot,
		StartUnixNano: time.Now().UTC().UnixNano(),
	}
}

type runningLocal struct {
	local  *yascheduler.Local
	cancel context.CancelFunc
	done   chan yaerrors.Error
}

func startLocal(
	t *testing.T,
	cfg *yascheduler.LocalConfig,
	registry *yascheduler.Registry,
) *runningLocal {
	t.Helper()

	local, err := yascheduler.NewLocal(cfg, registry, nil)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan yaerrors.Error, 1)

	go func() {
		done <- local.Run(ctx)
	}()

	t.Cleanup(cancel)

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer awaitCancel()

	if awaitErr := local.AwaitReady(awaitCtx); awaitErr != nil {
		t.Fatalf("AwaitReady failed: %v", awaitErr)
	}

	return &runningLocal{local: local, cancel: cancel, done: done}
}

func (rl *runningLocal) stop(t *testing.T) {
	t.Helper()

	rl.cancel()

	select {
	case err := <-rl.done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(localStopTimeout):
		t.Fatal("local Run did not stop in time")
	}
}

func registerLocalFunction(
	t *testing.T,
	registry *yascheduler.Registry,
	fn func(ctx context.Context, value int64) (int64, error),
) {
	t.Helper()

	if err := yascheduler.RegisterFunction(
		registry,
		localFunctionName,
		"",
		fn,
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}
}

func upsertLocalJob(
	t *testing.T,
	running *runningLocal,
	spec *yascheduler.JobSpec,
) protocol.JobUUID {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer cancel()

	submission, err := running.local.UpsertJob(ctx, spec)
	if err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	submission.Close()

	return submission.JobUUID
}

func TestLocalRunsJobEndToEnd(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()
	got := make(chan int64, 1)

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		got <- value

		return value, nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Engine:       fastLocalEngine(),
	}, registry)

	jobID := upsertLocalJob(t, running, &yascheduler.JobSpec{
		Key:      "end-to-end",
		Function: protocol.FunctionSpec{Name: localFunctionName},
		Args:     localArgValue,
		Schedule: oneShotNow(),
	})

	if jobID.IsZero() {
		t.Fatal("UpsertJob returned a zero job UUID")
	}

	select {
	case value := <-got:
		if value != localArgValue {
			t.Fatalf("function argument = %d, want %d", value, localArgValue)
		}
	case <-time.After(localExecuteTimeout):
		t.Fatal("function never executed")
	}

	running.stop(t)
}

func TestLocalDrainsRunningFunctionOnStop(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()
	started := make(chan struct{}, 1)

	var finished atomic.Bool

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		started <- struct{}{}

		time.Sleep(localDrainSleep)
		finished.Store(true)

		return value, nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		DrainTimeout: localDrainTimeout,
		Engine:       fastLocalEngine(),
	}, registry)

	upsertLocalJob(t, running, &yascheduler.JobSpec{
		Key:      "drain-on-stop",
		Function: protocol.FunctionSpec{Name: localFunctionName},
		Args:     localArgValue,
		Schedule: oneShotNow(),
	})

	select {
	case <-started:
	case <-time.After(localExecuteTimeout):
		t.Fatal("function never started")
	}

	running.cancel()

	select {
	case err := <-running.done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(localStopTimeout):
		t.Fatal("local Run did not stop in time")
	}

	if !finished.Load() {
		t.Fatal("Run returned before the running function finished")
	}
}

func TestLocalRedispatchesOnQueueFull(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()
	gate := make(chan struct{})
	executed := make(chan int64, localQueueFullBuffer)

	var startedCount atomic.Int32

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		startedCount.Add(1)

		<-gate

		executed <- value

		return value, nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Capacity:     localQueueFullCapacity,
		QueueSize:    1,
		Engine: engine.Config{
			Lease:             localQueueFullLease,
			MaxExecution:      localQueueFullLease,
			ReconcileInterval: localReconcileFast,
			RedispatchDelay:   localRedispatchFast,
		},
	}, registry)

	for index := range int64(localQueueFullJobs) {
		upsertLocalJob(t, running, &yascheduler.JobSpec{
			Key:      fmt.Sprintf("queue-full-%d", index),
			Function: protocol.FunctionSpec{Name: localFunctionName},
			Args:     index,
			Schedule: oneShotNow(),
		})
	}

	deadline := time.Now().Add(localExecuteTimeout)

	for startedCount.Load() < int32(localQueueFullCapacity) {
		if time.Now().After(deadline) {
			t.Fatalf(
				"only %d of %d capacity slots ever started",
				startedCount.Load(),
				localQueueFullCapacity,
			)
		}

		time.Sleep(localPollInterval)
	}

	close(gate)

	seen := make(map[int64]struct{}, localQueueFullJobs)

	for len(seen) < localQueueFullJobs {
		select {
		case value := <-executed:
			seen[value] = struct{}{}
		case <-time.After(localExecuteTimeout):
			t.Fatalf("only %d of %d jobs executed", len(seen), localQueueFullJobs)
		}
	}

	running.stop(t)
}

func TestLocalLabelAnnounceRoutesPinnedJob(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()
	executed := make(chan struct{}, 1)

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		executed <- struct{}{}

		return value, nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Engine:       fastLocalEngine(),
	}, registry)

	upsertLocalJob(t, running, &yascheduler.JobSpec{
		Key:      "pinned",
		Function: protocol.FunctionSpec{Name: localFunctionName},
		Args:     localArgValue,
		Schedule: oneShotNow(),
		Pin: protocol.PinSpec{
			Label:  localPinnedLabel,
			Policy: protocol.PinPolicyStrict,
		},
	})

	announceCtx, announceCancel := context.WithTimeout(
		context.Background(),
		localAwaitTimeout,
	)
	defer announceCancel()

	if err := running.local.AnnounceLabels(announceCtx, localPinnedLabel); err != nil {
		t.Fatalf("AnnounceLabels failed: %v", err)
	}

	select {
	case <-executed:
	case <-time.After(localExecuteTimeout):
		t.Fatal("pinned job never executed after the label was announced")
	}

	running.stop(t)
}

func TestLocalRequestResponseRoundTrip(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		return value * 2, nil
	})

	if err := yascheduler.RegisterFunction(
		registry,
		localVoidFunctionName,
		"",
		func(_ context.Context, _ int64) (yascheduler.Void, error) {
			return yascheduler.Void{}, nil
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Engine:       fastLocalEngine(),
	}, registry)

	upsertCtx, upsertCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer upsertCancel()

	submission, err := running.local.UpsertJob(upsertCtx, &yascheduler.JobSpec{
		Function:   protocol.FunctionSpec{Name: localFunctionName},
		Args:       localArgValue,
		Schedule:   oneShotNow(),
		ResultMode: protocol.ResultModeDeliver,
	})
	if err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), localExecuteTimeout)
	defer awaitCancel()

	result, awaitErr := submission.Await(awaitCtx)
	if awaitErr != nil {
		t.Fatalf("Await failed: %v", awaitErr)
	}

	if !result.Success || !result.HasValue {
		t.Fatalf(
			"result success = %t has value = %t, want a successful valued result",
			result.Success,
			result.HasValue,
		)
	}

	value, decodeErr := yascheduler.DecodeResult[int64](result)
	if decodeErr != nil {
		t.Fatalf("DecodeResult failed: %v", decodeErr)
	}

	if *value != localArgValue*2 {
		t.Fatalf("value = %d, want %d", *value, localArgValue*2)
	}

	voidSubmission, voidErr := running.local.UpsertJob(upsertCtx, &yascheduler.JobSpec{
		Function:   protocol.FunctionSpec{Name: localVoidFunctionName},
		Args:       localArgValue,
		Schedule:   oneShotNow(),
		ResultMode: protocol.ResultModeDeliver,
	})
	if voidErr != nil {
		t.Fatalf("void UpsertJob failed: %v", voidErr)
	}

	voidResult, voidAwaitErr := voidSubmission.Await(awaitCtx)
	if voidAwaitErr != nil {
		t.Fatalf("void Await failed: %v", voidAwaitErr)
	}

	if !voidResult.Success || voidResult.HasValue {
		t.Fatalf(
			"void result success = %t has value = %t, want success without a value",
			voidResult.Success,
			voidResult.HasValue,
		)
	}

	if _, err := yascheduler.DecodeResult[yascheduler.Void](voidResult); !errors.Is(
		err,
		yascheduler.ErrResultHasNoValue,
	) {
		t.Fatalf("void DecodeResult error = %v, want ErrResultHasNoValue", err)
	}

	running.stop(t)
}

func TestLocalIntervalDeliverAnswersFirstResult(t *testing.T) {
	t.Parallel()

	const (
		localIntervalMillis  = uint64(50)
		firstOccurrenceValue = int64(1)
		minOccurrences       = int32(2)
	)

	registry := yascheduler.NewRegistry()

	var occurrences atomic.Int32

	registerLocalFunction(t, registry, func(_ context.Context, _ int64) (int64, error) {
		return int64(occurrences.Add(1)), nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Engine:       fastLocalEngine(),
	}, registry)

	upsertCtx, upsertCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer upsertCancel()

	submission, err := running.local.UpsertJob(upsertCtx, &yascheduler.JobSpec{
		Key:      "interval-deliver",
		Function: protocol.FunctionSpec{Name: localFunctionName},
		Args:     localArgValue,
		Schedule: protocol.ScheduleSpec{
			Kind:           protocol.ScheduleKindFixedInterval,
			StartUnixNano:  time.Now().UTC().UnixNano(),
			IntervalMillis: localIntervalMillis,
		},
		Overlap:    protocol.OverlapPolicySkip,
		ResultMode: protocol.ResultModeDeliver,
	})
	if err != nil {
		t.Fatalf("UpsertJob failed: %v", err)
	}

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), localExecuteTimeout)
	defer awaitCancel()

	result, awaitErr := submission.Await(awaitCtx)
	if awaitErr != nil {
		t.Fatalf("Await failed: %v", awaitErr)
	}

	if !result.Success || !result.HasValue {
		t.Fatalf(
			"result success = %t has value = %t, want a successful valued result",
			result.Success,
			result.HasValue,
		)
	}

	value, decodeErr := yascheduler.DecodeResult[int64](result)
	if decodeErr != nil {
		t.Fatalf("DecodeResult failed: %v", decodeErr)
	}

	if *value != firstOccurrenceValue {
		t.Fatalf("value = %d, want the first occurrence's result", *value)
	}

	deadline := time.Now().Add(localExecuteTimeout)

	for occurrences.Load() < minOccurrences {
		if time.Now().After(deadline) {
			t.Fatalf(
				"only %d occurrences ran, want at least %d",
				occurrences.Load(),
				minOccurrences,
			)
		}

		time.Sleep(localPollInterval)
	}

	running.stop(t)
}

func TestLocalHeartbeatKeepsLongFunctionAlive(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()
	finished := make(chan struct{}, localQueueFullBuffer)

	var runs atomic.Int32

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		runs.Add(1)

		time.Sleep(localLongRuntime)

		finished <- struct{}{}

		return value, nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Engine: engine.Config{
			Lease:             localHeartbeatLease,
			ReconcileInterval: localReconcileFast,
			RedispatchDelay:   localRedispatchFast,
		},
	}, registry)

	upsertLocalJob(t, running, &yascheduler.JobSpec{
		Key:      "long-runner",
		Function: protocol.FunctionSpec{Name: localFunctionName},
		Args:     localArgValue,
		Schedule: oneShotNow(),
	})

	select {
	case <-finished:
	case <-time.After(localExecuteTimeout):
		t.Fatal("long function never finished")
	}

	time.Sleep(localSettleGrace)

	if got := runs.Load(); got != 1 {
		t.Fatalf("function ran %d times, want exactly 1: lease reaping redispatched it", got)
	}

	running.stop(t)
}

// pumpKillingStore panics on the attempt lookup the heartbeat path uses
// once armed, so a test can kill the local heartbeat pump through the
// public store seam and nothing else.
type pumpKillingStore struct {
	store.Store

	armed atomic.Bool
}

func (s *pumpKillingStore) AttemptsOnInstance(
	ctx context.Context,
	instanceID protocol.InstanceID,
	states ...store.AttemptState,
) ([]*store.Attempt, yaerrors.Error) {
	if s.armed.Load() {
		panic("induced heartbeat failure")
	}

	return s.Store.AttemptsOnInstance(ctx, instanceID, states...)
}

func TestLocalRunFailsWhenHeartbeatPumpDies(t *testing.T) {
	t.Parallel()

	registry := yascheduler.NewRegistry()

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		return value, nil
	})

	records := &pumpKillingStore{Store: memstore.NewStore(memstore.Config{})}

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Store:        records,
		Engine: engine.Config{
			Lease:             localPumpLease,
			ReconcileInterval: localReconcileIdle,
		},
	}, registry)

	records.armed.Store(true)

	select {
	case err := <-running.done:
		if err == nil {
			t.Fatal("Run returned nil after the heartbeat pump died")
		}

		if !errors.Is(err, yascheduler.ErrHeartbeatPumpStopped) {
			t.Fatalf("Run error = %v, want ErrHeartbeatPumpStopped", err)
		}
	case <-time.After(localExecuteTimeout):
		t.Fatal("Run kept serving after the heartbeat pump died")
	}
}
