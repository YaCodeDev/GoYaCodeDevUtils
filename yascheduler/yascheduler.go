// Package yascheduler is the executor-side library of the yascheduler
// distributed job scheduling system. An application service registers
// executable functions in a Registry, then runs a Scheduler that invokes
// them when executions come due.
//
// # Two implementations, one semantics
//
// The Scheduler interface has two implementations. Client connects to the
// standalone yascheduler service over raw TCP, registers this process as
// an executor, and serves dispatched executions across reconnects. Local
// runs the same scheduling engine in process: no service, no socket, no
// external store - jobs, executions, and attempts live in an in-memory
// store (or an injected store.Store) and dispatch crosses a bounded
// in-process loopback instead of a connection. Code written against
// Scheduler moves between the two without change.
//
// # Invocation design
//
// Functions are registered through the generic RegisterFunction, so the
// invocation wrapper is prepared entirely at registration time: argument
// decoding, the typed call, and result encoding are compiled into one
// closure with no reflection on the execution path. Compared to a
// reflect.Value.Call-based dispatcher this is both safer - the function
// shape is checked by the compiler, not at runtime - and faster, because
// every execution is a plain closure call plus MessagePack codec work.
// The only reflection happens once per registration, to derive the
// signature strings advertised to the scheduler, and its result is cached
// in the registered spec. Malformed scheduler input can never crash the
// executor: argument decoding failures become structured execution
// errors, and function panics are recovered and reported as such.
//
// # Delivery semantics
//
// The system provides at-least-once execution. If an executor disconnects
// after accepting an execution but before its result frame is delivered,
// the scheduler redispatches the execution after its lease expires, so a
// function may run more than once for the same occurrence. Local mode
// keeps the same contract in process: a dispatch refused by a full
// loopback queue is requeued and redispatched, and a lost result is
// reclaimed by the same lease expiry. Every ExecRequest carries stable
// JobUUID, ExecutionID and AttemptID values, and the runtime exposes them
// to the running function as the Invocation on its context; handlers that
// cause external effects must read InvocationFromContext and use its
// ExecutionID (stable across redispatches of the same occurrence) as an
// idempotency key.
//
// # Request and response
//
// UpsertJob returns a Submission. By default a job's result is ignored
// (protocol.ResultModeIgnore) and Await answers ErrResultNotRequested;
// setting JobSpec.ResultMode to protocol.ResultModeDeliver makes the
// scheduler hold the final result and deliver it back to the submitting
// process, where Await blocks for it. An empty JobSpec.Key submits an
// RPC-style one-shot keyed by the minted job UUID, so the call pattern
// "upsert with Deliver, then Await, then DecodeResult" is a remote
// function call. A caller that stops caring calls Close, which releases
// the registered waiter; Await closes the submission on every return
// path, so one submission answers at most one result.
//
// Result delivery is itself at-least-once: the scheduler holds a result
// until the submitter acknowledges it, redelivering across reconnects
// and instance restarts within the scheduler's retention budget. The
// per-job waiter buffers exactly one result and discards duplicates, and
// a delivery finding no waiter is refused so the scheduler stops
// redelivering it. A function with nothing to return uses Void as its
// result type: its result arrives with no value, and DecodeResult on it
// answers ErrResultHasNoValue.
//
// A repeating schedule composes with Deliver through that same hold: the
// scheduler keeps at most one held result per job, so each settled
// occurrence's result replaces a still-held undelivered one while
// keeping its delivery counters. Await answers the first delivered
// result; once the submission is closed, a later occurrence's delivery
// finds no waiter, is refused, and is dropped with the held entry.
//
// Re-upserting an existing key replaces the job definition, and the
// pending occurrence survives the replacement only when it falls a whole
// number of intervals after the replacement schedule's anchor - so keep a
// repeating job's StartUnixNano stable across republishes rather than
// re-anchoring it to the current time.
//
// # Persistence
//
// Local runs on an in-memory store by default, so jobs die with the
// process. Supplying LocalConfig.Store - store/redisstore over any
// redis-protocol server, Dragonfly included - makes jobs, executions,
// attempts, and held results survive a restart: a new Local over the
// same backend finds the stored jobs and fires them without a
// re-upsert, abandoning interrupted attempts into redispatch. What does
// not survive is the result waiter registry: it is in-memory per
// runtime, so a pending Await dies with its process, while a
// scheduler-side held result survives the restart and is redelivered or
// expired under the engine's retention budget.
package yascheduler

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// Scheduler is the caller-facing surface of one scheduling runtime,
// remote or in-process. Run serves until its context ends; every other
// method may be called from any goroutine once Run has been started.
type Scheduler interface {
	// Run serves executions until ctx is cancelled, then drains running
	// functions before returning.
	Run(ctx context.Context) yaerrors.Error

	// AwaitReady blocks until this scheduler accepts work or ctx ends.
	AwaitReady(ctx context.Context) yaerrors.Error

	// UpsertJob creates or updates the job identified by spec.Key within
	// its executor type and returns the submission handle for it; an
	// empty key submits an RPC-style one-shot keyed by the minted job
	// UUID. Under ResultModeDeliver the submission awaits the delivered
	// result.
	UpsertJob(ctx context.Context, spec *JobSpec) (*Submission, yaerrors.Error)

	// DeleteJob withdraws the job addressed by key within the given
	// executor type; an empty executor type addresses this scheduler's
	// own. Pending occurrences are cancelled, a held result is dropped,
	// and the key is freed for a fresh job, while work already running
	// finishes on its own. Deleting an absent job reports false with no
	// error, so a replayed delete is idempotent.
	DeleteJob(
		ctx context.Context,
		executorType protocol.ExecutorType,
		key string,
	) (bool, yaerrors.Error)

	// AnnounceLabels adds routing labels to the set this executor holds,
	// so jobs pinned to them may route here.
	AnnounceLabels(ctx context.Context, labels ...protocol.Label) yaerrors.Error

	// WithdrawLabels removes routing labels from the set this executor
	// holds. Attempts already dispatched under a withdrawn label run to
	// completion: labels bind at dispatch.
	WithdrawLabels(ctx context.Context, labels ...protocol.Label) yaerrors.Error

	// InstanceID returns the stable process instance identity this
	// scheduler registers and submits under.
	InstanceID() protocol.InstanceID
}

var (
	_ Scheduler = (*Client)(nil)
	_ Scheduler = (*Local)(nil)
)
