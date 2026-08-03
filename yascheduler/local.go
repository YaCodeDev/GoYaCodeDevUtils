package yascheduler

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/memstore"
	"github.com/google/uuid"
)

// LocalConfig configures one in-process scheduler. ExecutorType is
// required; every other field falls back to a package default.
type LocalConfig struct {
	// ExecutorType names what kind of service this process is.
	ExecutorType protocol.ExecutorType

	// InstanceID identifies this process. Leave empty to generate one at
	// construction.
	InstanceID protocol.InstanceID

	// Capacity bounds concurrent executions on this executor.
	Capacity uint32

	// DrainTimeout bounds how long a stopping scheduler waits for running
	// functions before cancelling their contexts.
	DrainTimeout time.Duration

	// QueueSize bounds each of the two loopback queues between the engine
	// and the executor runtime.
	QueueSize int

	// Labels are the routing labels this executor announces at startup.
	Labels []protocol.Label

	// Engine tunes the embedded scheduling engine.
	Engine engine.Config

	// Store persists jobs, executions, attempts, and results. Leave nil
	// for an in-memory store with package defaults.
	Store store.Store
}

// normalized validates the required fields and returns a copy with every
// zero field filled with its package default, generating an InstanceID
// when none is given.
func (c *LocalConfig) normalized() (LocalConfig, yaerrors.Error) {
	normalized := *c
	if normalized.ExecutorType == "" {
		return normalized, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyExecutorType,
			logTag+" local config",
		)
	}

	if normalized.InstanceID == "" {
		normalized.InstanceID = protocol.InstanceID(uuid.NewString())
	}

	if normalized.Capacity == 0 {
		normalized.Capacity = DefaultCapacity
	}

	if normalized.DrainTimeout <= 0 {
		normalized.DrainTimeout = DefaultDrainTimeout
	}

	if normalized.QueueSize <= 0 {
		normalized.QueueSize = DefaultOutgoingQueueSize
	}

	return normalized, nil
}

// Local runs the yascheduler scheduling engine, its store, and one
// executor runtime inside a single process: no service, no socket, no
// external store. It provides the same at-least-once semantics as the
// remote path - a dispatch refused by a full loopback queue is requeued
// and redispatched, and a lost result is reclaimed by lease expiry.
type Local struct {
	cfg LocalConfig
	log yalogger.Logger

	runtime      executorRuntime
	loopback     *loopback
	engine       engine.Engine
	records      store.Store
	execRegistry engine.ExecutorRegistry

	heartbeatInterval time.Duration

	running   atomic.Bool
	ready     chan struct{}
	readyOnce sync.Once
}

// NewLocal builds an in-process scheduler over cfg and the given function
// registry. A nil log falls back to the base yalogger logger.
func NewLocal(
	cfg *LocalConfig,
	registry *Registry,
	log yalogger.Logger,
) (*Local, yaerrors.Error) {
	if registry == nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilRegistry,
			logTag+" new local scheduler",
		)
	}

	if cfg == nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilConfig,
			logTag+" new local scheduler",
		)
	}

	normalized, err := cfg.normalized()
	if err != nil {
		return nil, err.Wrap(logTag + " new local scheduler")
	}

	if log == nil {
		log = yalogger.NewBaseLogger(nil).NewLogger()
	}

	log = log.WithField("component", "yascheduler-local")

	records := normalized.Store
	if records == nil {
		records = memstore.NewStore(memstore.Config{})
	}

	execRegistry := engine.NewExecutorRegistry()

	local := &Local{
		cfg: normalized,
		log: log,
		runtime: executorRuntime{
			registry: registry,
			log:      log,
			results: resultRegistry{
				waiters: make(map[protocol.JobUUID]chan *Result),
			},
			execSlots: make(chan struct{}, normalized.Capacity),
			cancels:   make(map[protocol.ExecutionID]map[cancelToken]context.CancelFunc),
		},
		loopback: newLoopback(normalized.QueueSize, log),
		engine: engine.NewEngine(
			&normalized.Engine,
			records,
			records,
			records,
			records,
			execRegistry,
			log,
		),
		records:           records,
		execRegistry:      execRegistry,
		heartbeatInterval: localHeartbeatInterval(normalized.Engine.Lease),
		ready:             make(chan struct{}),
	}

	local.runtime.sink = local.loopback.enqueueToEngine

	return local, nil
}

// localHeartbeatInterval derives the pump cadence from the engine lease,
// so several ticks fit inside every lease window.
func localHeartbeatInterval(lease time.Duration) (interval time.Duration) {
	if lease <= 0 {
		lease = engine.DefaultLease
	}

	return lease / heartbeatsPerLease
}

// InstanceID returns the stable process instance ID this scheduler
// registers and submits under.
func (l *Local) InstanceID() protocol.InstanceID {
	return l.cfg.InstanceID
}

// Run starts the engine and the loopback, registers this process as the
// engine's executor, and serves until ctx is cancelled. On cancellation it
// pauses dispatch, drains running functions for up to DrainTimeout,
// cancels the leftovers and waits one more DrainTimeout, then stops the
// engine and the loopback. Run may be called once per Local.
func (l *Local) Run(ctx context.Context) yaerrors.Error {
	if !l.running.CompareAndSwap(false, true) {
		return yaerrors.FromError(
			http.StatusConflict,
			ErrLocalAlreadyRunning,
			logTag+" run local",
		)
	}

	defer l.running.Store(false)

	if l.loopback.stopped() {
		return yaerrors.FromError(
			http.StatusConflict,
			ErrLoopbackStopped,
			logTag+" run local",
		)
	}

	l.runtime.mu.Lock()
	l.runtime.stopping.Store(false)
	l.runtime.mu.Unlock()

	execCtx, execCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer execCancel()

	drainCtx := context.WithoutCancel(ctx)

	l.engine.Start(ctx)

	l.loopback.start(
		func(msg protocol.Message) { l.routeToExecutor(execCtx, msg) },
		func(msg protocol.Message) { l.routeToEngine(drainCtx, msg) },
	)

	l.execRegistry.Register(
		l.cfg.InstanceID,
		l.cfg.ExecutorType,
		store.Capacity(l.cfg.Capacity),
		l.runtime.registry.specs(),
		l.cfg.Labels,
		l.loopback,
	)

	l.engine.HandleRegistered(drainCtx, l.cfg.InstanceID)

	pumpDone := make(chan struct{})

	go l.heartbeatPump(ctx, pumpDone)

	l.readyOnce.Do(func() { close(l.ready) })

	l.log.WithFields(map[string]any{
		"executor_type": string(l.cfg.ExecutorType),
		"instance_id":   string(l.cfg.InstanceID),
	}).Info(logTag + " local scheduler started")

	pumpDied := false

	select {
	case <-ctx.Done():
	case <-pumpDone:
		if ctx.Err() == nil {
			pumpDied = true
		}
	}

	l.engine.Pause()
	l.runtime.beginShutdown()

	drained := l.runtime.awaitExecutions(l.cfg.DrainTimeout)
	if !drained {
		l.log.Warn(logTag + " drain timeout exceeded, cancelling running functions")
		execCancel()

		drained = l.runtime.awaitExecutions(l.cfg.DrainTimeout)
	}

	stopCtx, stopCancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		l.cfg.DrainTimeout,
	)
	defer stopCancel()

	l.engine.Stop(stopCtx)
	l.loopback.Stop()

	<-pumpDone

	if pumpDied {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrHeartbeatPumpStopped,
			logTag+" run local",
		)
	}

	if !drained {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrDrainTimeout,
			logTag+" run local",
		)
	}

	return nil
}

// heartbeatPump renews the leases of running work the way a connected
// executor's heartbeats would. Its death must fail Run: if heartbeats
// silently stop, lease reaping redispatches executions still running in
// this process, which is a double execution inside one binary. A panic
// escaping a heartbeat is therefore recovered, logged, and surfaced as
// pump death rather than swallowed.
func (l *Local) heartbeatPump(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer func() {
		if reason := recover(); reason != nil {
			l.log.Errorf(logTag+" heartbeat pump panicked: %v", reason)
		}
	}()

	ticker := time.NewTicker(l.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if entry, found := l.execRegistry.Get(l.cfg.InstanceID); found {
				entry.Heartbeat(time.Now().UTC())
			}

			l.engine.HandleHeartbeat(ctx, l.cfg.InstanceID)
		}
	}
}

// AwaitReady blocks until Run has registered this process with the engine
// or ctx ends.
func (l *Local) AwaitReady(ctx context.Context) yaerrors.Error {
	select {
	case <-l.ready:
		return nil
	case <-ctx.Done():
		return yaerrors.FromError(
			http.StatusServiceUnavailable,
			ctx.Err(),
			logTag+" await ready",
		)
	}
}

// UpsertJob creates or updates the job identified by spec.Key within its
// executor type on the embedded engine and returns the submission handle
// for it; an empty
// spec.Key submits an RPC-style one-shot keyed by the minted job UUID.
// Under ResultModeDeliver the result waiter is registered before the
// upsert reaches the engine, because in process the result of a fast
// function can settle before this call returns. Signature fields left
// empty on spec.Function are stamped from the local registry, and a
// backfill mode of BackfillModeInherit falls through to the engine
// default.
func (l *Local) UpsertJob(
	ctx context.Context,
	spec *JobSpec,
) (*Submission, yaerrors.Error) {
	upsert, err := buildJobUpsert(
		spec,
		l.runtime.registry,
		l.cfg.ExecutorType,
		protocol.BackfillModeInherit,
	)
	if err != nil {
		return nil, err.Wrap(logTag + " upsert job")
	}

	submission := l.runtime.results.open(upsert.JobUUID, upsert.ResultMode)

	ack := l.engine.HandleJobUpsert(ctx, l.cfg.InstanceID, upsert)
	if !ack.Accepted {
		submission.Close()

		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrUpsertRejected,
			logTag+" upsert job: "+wireErrorText(ack.Error),
		)
	}

	submission.adopt(ack.JobUUID)

	return submission, nil
}

// DeleteJob withdraws the job addressed by key within the given executor
// type on the embedded engine; an empty executor type addresses this
// scheduler's own. The engine cancels the job's pending occurrences, drops
// a held result, and frees the key for a fresh job, while work already
// running finishes on its own. Deleting an absent job reports false with
// no error, so a replayed delete is idempotent.
func (l *Local) DeleteJob(
	ctx context.Context,
	executorType protocol.ExecutorType,
	key string,
) (bool, yaerrors.Error) {
	if executorType == "" {
		executorType = l.cfg.ExecutorType
	}

	ack := l.engine.HandleJobDelete(ctx, l.cfg.InstanceID, &protocol.JobDelete{
		JobKey:       key,
		ExecutorType: executorType,
	})
	if ack.Error != nil {
		return false, yaerrors.FromError(
			http.StatusBadRequest,
			ErrDeleteRejected,
			logTag+" delete job: "+wireErrorText(ack.Error),
		)
	}

	return ack.Deleted, nil
}

// AnnounceLabels adds routing labels to the set this executor announces,
// waking any job pinned to them.
func (l *Local) AnnounceLabels(
	ctx context.Context,
	labels ...protocol.Label,
) yaerrors.Error {
	return l.updateLabels(ctx, labels, nil)
}

// WithdrawLabels removes routing labels from the set this executor
// announces. Attempts already dispatched under a withdrawn label run to
// completion: labels bind at dispatch.
func (l *Local) WithdrawLabels(
	ctx context.Context,
	labels ...protocol.Label,
) yaerrors.Error {
	return l.updateLabels(ctx, nil, labels)
}

// updateLabels applies one label revision through the engine, so local
// callers get exactly the admission the wire path enforces.
func (l *Local) updateLabels(
	ctx context.Context,
	announce []protocol.Label,
	withdraw []protocol.Label,
) yaerrors.Error {
	ack := l.engine.HandleLabelUpdate(ctx, l.cfg.InstanceID, &protocol.LabelUpdate{
		Announce: announce,
		Withdraw: withdraw,
	})
	if !ack.Accepted {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrLabelUpdateRejected,
			logTag+" update labels: "+wireErrorText(ack.Error),
		)
	}

	return nil
}
