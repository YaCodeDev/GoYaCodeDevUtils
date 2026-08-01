// Package engine implements the scheduling core of yascheduler: it holds
// the connected executors, decides which of them runs a due occurrence, and
// settles the outcome. A transport owns connections and message framing and
// calls the Handle methods here; the engine itself speaks no network.
//
// # Label-pinned routing
//
// A job may pin itself to a routing label, and an executor announces the
// labels it holds at registration and revises them with LabelUpdate. A label
// refines routing inside a pool: a pinned job still has to match the
// executor type and the function spec, and the label only narrows which of
// the matching executors may take it. A strict pin waits for its label; a
// preferred pin widens back to the whole pool once the label has no taker.
//
// # Labels bind at dispatch
//
// A label is consulted when an attempt is handed out and never again. An
// attempt already dispatched to an executor runs to completion even if that
// executor withdraws the label a moment later, and the withdrawal is only
// logged and counted. Closing the remaining window — selection, withdrawal,
// and enqueue interleaving on separate goroutines — would need a two-phase
// commit across dispatch, which buys less than it costs: cancelling
// in-flight work on withdrawal turns a routing decision into a
// duplicate-execution race against the redispatch, whereas letting the
// attempt finish is self-healing. If the resource behind the label really
// moved, the function fails retryably and the retry routes to the new
// holder.
package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

var _ Engine = (*engine)(nil)

type engine struct {
	jobs       store.JobRepository
	executions store.ExecutionRepository
	attempts   store.AttemptRepository
	results    store.ResultRepository
	registry   ExecutorRegistry
	cfg        Config
	metrics    *Metrics
	log        yalogger.Logger

	notifyCh chan struct{}
	stopping atomic.Bool

	lifecycleMu   sync.Mutex
	cancel        context.CancelFunc
	loopDone      chan struct{}
	reconcileDone chan struct{}

	clockMu sync.RWMutex
	clock   Clock
}

// NewEngine builds a scheduling engine over the given stores and executor
// registry. A nil config configures every field with its package default.
func NewEngine(
	cfg *Config,
	jobs store.JobRepository,
	executions store.ExecutionRepository,
	attempts store.AttemptRepository,
	results store.ResultRepository,
	registry ExecutorRegistry,
	log yalogger.Logger,
) (created Engine) {
	if cfg == nil {
		cfg = &Config{}
	}

	return &engine{
		jobs:       jobs,
		executions: executions,
		attempts:   attempts,
		results:    results,
		registry:   registry,
		cfg:        cfg.normalized(),
		metrics:    &Metrics{},
		log:        log,
		notifyCh:   make(chan struct{}, notifyBuffer),
		clock:      func() time.Time { return time.Now().UTC() },
	}
}

func (e *engine) Start(ctx context.Context) {
	e.lifecycleMu.Lock()

	if e.cancel != nil {
		e.lifecycleMu.Unlock()

		return
	}

	runCtx, cancel := context.WithCancel(ctx)

	loopDone := make(chan struct{})
	reconcileDone := make(chan struct{})

	e.cancel = cancel
	e.loopDone = loopDone
	e.reconcileDone = reconcileDone

	e.lifecycleMu.Unlock()

	e.stopping.Store(false)

	e.registry.SetNotify(func(change RegistryChange) {
		e.reconsiderWaiting(runCtx, change)
	})

	e.recoverOnStartup(runCtx)

	go e.run(runCtx, loopDone)
	go e.reconcileLoop(runCtx, reconcileDone)
}

func (e *engine) Pause() {
	e.stopping.Store(true)
}

func (e *engine) Stop(ctx context.Context) {
	e.stopping.Store(true)

	e.lifecycleMu.Lock()

	cancel := e.cancel
	loopDone := e.loopDone
	reconcileDone := e.reconcileDone

	e.cancel = nil
	e.loopDone = nil
	e.reconcileDone = nil

	e.lifecycleMu.Unlock()

	if cancel == nil {
		return
	}

	cancel()

	for _, done := range []chan struct{}{loopDone, reconcileDone} {
		select {
		case <-done:
		case <-ctx.Done():
			e.log.Warn(logTag + " drain timed out before completion")

			return
		}
	}
}

func (e *engine) Notify() {
	select {
	case e.notifyCh <- struct{}{}:
	default:
	}
}

func (e *engine) SetClock(now Clock) {
	e.clockMu.Lock()
	defer e.clockMu.Unlock()

	e.clock = now
}

func (e *engine) now() (now time.Time) {
	e.clockMu.RLock()
	defer e.clockMu.RUnlock()

	return e.clock().UTC()
}

func (e *engine) Snapshot() (snapshot map[string]uint64) {
	return e.metrics.Snapshot()
}

func (e *engine) run(ctx context.Context, done chan struct{}) {
	defer close(done)

	e.log.Info(logTag + " timing loop started")

	for {
		if ctx.Err() != nil {
			e.log.Info(logTag + " timing loop stopped")

			return
		}

		if !e.stopping.Load() {
			e.processDue(ctx)
		}

		var (
			timer  *time.Timer
			wakeCh <-chan time.Time
		)

		next, hasNext, err := e.executions.NextWakeAt(ctx)
		if err != nil {
			e.log.Errorf(logTag+" next wake lookup failed: %v", err)
		} else if hasNext {
			delay := next.Sub(e.now())
			if delay < 0 {
				delay = 0
			}

			timer = time.NewTimer(delay)
			wakeCh = timer.C
		}

		select {
		case <-ctx.Done():
		case <-e.notifyCh:
		case <-wakeCh:
		}

		if timer != nil {
			timer.Stop()
		}
	}
}

func (e *engine) reconcileLoop(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(e.cfg.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.reconcile(ctx)
		}
	}
}

func (e *engine) reconcile(ctx context.Context) {
	if e.stopping.Load() {
		return
	}

	now := e.now()

	e.reapExpiredLeases(ctx, now)
	e.requeueWaitingWithPools(ctx)
	e.ensurePendingOccurrences(ctx, now)
	e.sweepResults(ctx, now)

	e.log.WithFields(metricsFields(e.metrics.Snapshot())).
		Debug(logTag + " reconcile pass finished")
}

func (e *engine) reapExpiredLeases(ctx context.Context, now time.Time) {
	expired, err := e.executions.ExpiredLeases(ctx, now)
	if err != nil {
		e.log.Errorf(logTag+" expired lease lookup failed: %v", err)

		return
	}

	for _, execution := range expired {
		e.abandonAttempt(ctx, execution, lostReasonLease)
	}

	if len(expired) > 0 {
		e.Notify()
	}
}

func (e *engine) requeueWaitingWithPools(ctx context.Context) {
	waiting, err := e.executions.ExecutionsInStates(
		ctx,
		store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
	)
	if err != nil {
		e.log.Errorf(logTag+" waiting executions lookup failed: %v", err)

		return
	}

	requeued := false

	for _, execution := range waiting {
		job, jobErr := e.jobs.GetJob(ctx, execution.JobID)
		if jobErr != nil {
			continue
		}

		if e.registry.PoolSize(job.ExecutorType) == 0 {
			continue
		}

		if e.requeueReady(ctx, execution.ID, 0) {
			requeued = true
		}
	}

	if requeued {
		e.Notify()
	}
}

func (e *engine) ensurePendingOccurrences(ctx context.Context, now time.Time) {
	jobs, err := e.jobs.ListEnabledJobs(ctx)
	if err != nil {
		e.log.Errorf(logTag+" enabled jobs lookup failed: %v", err)

		return
	}

	created := false

	for _, job := range jobs {
		pending, pendingErr := e.executions.HasPendingOccurrence(ctx, job.ID)
		if pendingErr != nil || pending {
			continue
		}

		occurrence, exists := nextOccurrence(job.Schedule, now)
		if !exists {
			continue
		}

		_, wasCreated, createErr := e.executions.CreateExecution(
			ctx,
			job.ID,
			occurrence,
			store.StateScheduled,
			false,
		)
		if createErr == nil && wasCreated {
			created = true
		}
	}

	if created {
		e.Notify()
	}
}
