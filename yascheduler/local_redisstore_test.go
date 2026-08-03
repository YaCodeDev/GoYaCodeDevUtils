package yascheduler_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/redisstore"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	redisSubmitterInstanceID protocol.InstanceID = "redis-restart-submitter"

	redisRestartJobKey    = "redis-restart-periodic"
	redisDeleteJobKey     = "redis-restart-deleted"
	redisHeldResultJobKey = "redis-held-result"

	redisPeriodicInterval = 50 * time.Millisecond

	redisGateReleaseDelay = 2 * time.Second
	redisQuietWindow      = 300 * time.Millisecond
)

func newRedisStore(t *testing.T, server *miniredis.Miniredis) *redisstore.Store {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	return redisstore.NewStore(client, redisstore.Config{})
}

func redisPeriodicSpec(key string) *yascheduler.JobSpec {
	return &yascheduler.JobSpec{
		Key:      key,
		Function: protocol.FunctionSpec{Name: localFunctionName},
		Args:     localArgValue,
		Schedule: protocol.ScheduleSpec{
			Kind:           protocol.ScheduleKindFixedInterval,
			StartUnixNano:  time.Now().UTC().UnixNano(),
			IntervalMillis: uint64(redisPeriodicInterval / time.Millisecond),
		},
	}
}

func awaitCounter(t *testing.T, counter *atomic.Int64, want int64, message string) {
	t.Helper()

	deadline := time.Now().Add(localExecuteTimeout)

	for counter.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("%s: got %d, want at least %d", message, counter.Load(), want)
		}

		time.Sleep(localPollInterval)
	}
}

func TestLocalRedisStoreDeliversResultEndToEnd(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	registry := yascheduler.NewRegistry()

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		return value * 2, nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Store:        newRedisStore(t, server),
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

	running.stop(t)
}

func TestLocalRedisStoreJobSurvivesRestart(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)

	var firstCalls atomic.Int64

	firstRegistry := yascheduler.NewRegistry()

	registerLocalFunction(t, firstRegistry, func(_ context.Context, value int64) (int64, error) {
		firstCalls.Add(1)

		return value, nil
	})

	first := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Store:        newRedisStore(t, server),
		Engine:       fastLocalEngine(),
	}, firstRegistry)

	jobID := upsertLocalJob(t, first, redisPeriodicSpec(redisRestartJobKey))

	awaitCounter(t, &firstCalls, 1, "the periodic job should fire before the restart")

	first.stop(t)

	restartStore := newRedisStore(t, server)

	lookupCtx, lookupCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer lookupCancel()

	job, jobErr := restartStore.GetJobByKey(
		lookupCtx,
		localExecutorType,
		redisRestartJobKey,
	)
	if jobErr != nil {
		t.Fatalf("GetJobByKey after the restart failed: %v", jobErr)
	}

	if job.ID != jobID {
		t.Fatalf(
			"stored job UUID after the restart = %s, want %s: the job identity should survive",
			job.ID,
			jobID,
		)
	}

	if !bool(job.Enabled) {
		t.Fatal("the surviving job should still be enabled")
	}

	var secondCalls atomic.Int64

	secondRegistry := yascheduler.NewRegistry()

	registerLocalFunction(t, secondRegistry, func(_ context.Context, value int64) (int64, error) {
		secondCalls.Add(1)

		return value, nil
	})

	second := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Store:        restartStore,
		Engine:       fastLocalEngine(),
	}, secondRegistry)

	awaitCounter(
		t,
		&secondCalls,
		1,
		"the surviving job should fire under the restarted scheduler without a re-upsert",
	)

	second.stop(t)
}

func TestLocalRedisStoreDeleteJobPersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)

	var firstCalls atomic.Int64

	firstRegistry := yascheduler.NewRegistry()

	registerLocalFunction(t, firstRegistry, func(_ context.Context, value int64) (int64, error) {
		firstCalls.Add(1)

		return value, nil
	})

	first := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Store:        newRedisStore(t, server),
		Engine:       fastLocalEngine(),
	}, firstRegistry)

	upsertLocalJob(t, first, redisPeriodicSpec(redisDeleteJobKey))

	awaitCounter(t, &firstCalls, 1, "the periodic job should fire before the delete")

	deleteCtx, deleteCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer deleteCancel()

	deleted, deleteErr := first.local.DeleteJob(deleteCtx, "", redisDeleteJobKey)
	if deleteErr != nil {
		t.Fatalf("DeleteJob failed: %v", deleteErr)
	}

	if !deleted {
		t.Fatal("deleting the stored job should report true")
	}

	first.stop(t)

	restartStore := newRedisStore(t, server)

	lookupCtx, lookupCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer lookupCancel()

	if _, lookupErr := restartStore.GetJobByKey(
		lookupCtx,
		localExecutorType,
		redisDeleteJobKey,
	); !errors.Is(lookupErr, store.ErrJobNotFound) {
		t.Fatalf("GetJobByKey after the restart error = %v, want ErrJobNotFound", lookupErr)
	}

	var secondCalls atomic.Int64

	secondRegistry := yascheduler.NewRegistry()

	registerLocalFunction(t, secondRegistry, func(_ context.Context, value int64) (int64, error) {
		secondCalls.Add(1)

		return value, nil
	})

	second := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		Store:        restartStore,
		Engine:       fastLocalEngine(),
	}, secondRegistry)

	time.Sleep(redisQuietWindow)

	if got := secondCalls.Load(); got != 0 {
		t.Fatalf("a deleted job fired %d executions after the restart, want none", got)
	}

	replayCtx, replayCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer replayCancel()

	replayed, replayErr := second.local.DeleteJob(replayCtx, "", redisDeleteJobKey)
	if replayErr != nil {
		t.Fatalf("a replayed DeleteJob after the restart failed: %v", replayErr)
	}

	if replayed {
		t.Fatal("a replayed delete after the restart should report false")
	}

	second.stop(t)
}

// resultGateStore parks the engine's result-capture goroutine inside
// StoreResult after the underlying store has persisted the entry, so a test
// can hold the settled result stored-but-undelivered until the loopback has
// stopped and the delivery enqueue is refused.
type resultGateStore struct {
	store.Store

	stored     chan struct{}
	release    chan struct{}
	storedOnce sync.Once
}

func (s *resultGateStore) StoreResult(
	ctx context.Context,
	result *store.PendingResult,
) (bool, yaerrors.Error) {
	stored, err := s.Store.StoreResult(ctx, result)

	s.storedOnce.Do(func() { close(s.stored) })

	<-s.release

	return stored, err
}

func TestLocalRedisStoreHeldResultSurvivesRestart(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)

	records := &resultGateStore{
		Store:   newRedisStore(t, server),
		stored:  make(chan struct{}),
		release: make(chan struct{}),
	}

	registry := yascheduler.NewRegistry()

	registerLocalFunction(t, registry, func(_ context.Context, value int64) (int64, error) {
		return value * 2, nil
	})

	running := startLocal(t, &yascheduler.LocalConfig{
		ExecutorType: localExecutorType,
		InstanceID:   redisSubmitterInstanceID,
		Store:        records,
		Engine: engine.Config{
			ReconcileInterval: localReconcileIdle,
			RedispatchDelay:   localRedispatchFast,
		},
	}, registry)

	jobID := upsertLocalJob(t, running, &yascheduler.JobSpec{
		Key:        redisHeldResultJobKey,
		Function:   protocol.FunctionSpec{Name: localFunctionName},
		Args:       localArgValue,
		Schedule:   oneShotNow(),
		ResultMode: protocol.ResultModeDeliver,
	})

	select {
	case <-records.stored:
	case <-time.After(localExecuteTimeout):
		t.Fatal("the delivered result was never stored")
	}

	running.cancel()

	time.Sleep(redisGateReleaseDelay)
	close(records.release)

	select {
	case err := <-running.done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(localStopTimeout):
		t.Fatal("local Run did not stop in time")
	}

	restartStore := newRedisStore(t, server)

	heldCtx, heldCancel := context.WithTimeout(context.Background(), localAwaitTimeout)
	defer heldCancel()

	held, heldErr := restartStore.ResultsForInstance(heldCtx, redisSubmitterInstanceID, 0)
	if heldErr != nil {
		t.Fatalf("ResultsForInstance after the restart failed: %v", heldErr)
	}

	if len(held) != 1 {
		t.Fatalf(
			"held results after the restart = %d, want exactly 1 surviving undelivered result",
			len(held),
		)
	}

	survivor := held[0]

	if survivor.JobUUID != jobID {
		t.Fatalf("held result job UUID = %s, want %s", survivor.JobUUID, jobID)
	}

	if !bool(survivor.Success) || !bool(survivor.HasValue) {
		t.Fatalf(
			"held result success = %t has value = %t, want a successful valued result",
			bool(survivor.Success),
			bool(survivor.HasValue),
		)
	}

	value, decodeErr := yaencoding.DecodeMessagePack[int64]([]byte(survivor.Payload))
	if decodeErr != nil {
		t.Fatalf("held result payload decode failed: %v", decodeErr)
	}

	if *value != localArgValue*2 {
		t.Fatalf("held result value = %d, want %d", *value, localArgValue*2)
	}
}
