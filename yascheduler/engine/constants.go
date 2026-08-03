package engine

import (
	"math"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// Engine defaults, applied wherever a Config field is zero or negative.
const (
	// DefaultLease is how long a dispatched or running execution holds its
	// lease before a reconcile pass reclaims it.
	DefaultLease = 30 * time.Second

	// DefaultMaxExecution caps how long heartbeats keep renewing the lease
	// of one attempt, so a wedged executor cannot hold an execution open
	// forever.
	DefaultMaxExecution = time.Hour

	// DefaultReconcileInterval is the cadence of the reconcile pass.
	DefaultReconcileInterval = 5 * time.Second

	// DefaultRedispatchDelay delays the requeue of an execution whose
	// delivery failed for an infrastructure reason.
	DefaultRedispatchDelay = 250 * time.Millisecond

	// DefaultRetryInitialDelay seeds the function-error retry delay of a
	// job that states none of its own.
	DefaultRetryInitialDelay = time.Second

	// DefaultRetryMaxDelay caps the function-error retry delay of a job
	// that states none of its own.
	DefaultRetryMaxDelay = time.Minute

	// DefaultBackfillMaxAge caps how old a missed occurrence may be and
	// still be materialized.
	DefaultBackfillMaxAge = 24 * time.Hour

	// DefaultResultRetention bounds how long a settled result is held for
	// its submitter before a reconcile pass evicts it.
	DefaultResultRetention = 5 * time.Minute

	// DefaultRetryMultiplier grows the exponential retry delay of a job
	// whose spec carries no usable multiplier.
	DefaultRetryMultiplier = 2.0
)

// DefaultDispatchBatch caps how many due executions one timing-loop pass
// dispatches.
const DefaultDispatchBatch store.BatchLimit = 256

// DefaultBackfillMaxCount caps how many missed occurrences one job
// materializes.
const DefaultBackfillMaxCount store.OccurrenceCount = 100

// DefaultMaxPendingResults caps how many settled results one engine holds
// across every submitter.
const DefaultMaxPendingResults store.OccurrenceCount = 1024

// DefaultMaxPendingResultsPerInstance caps how many settled results one
// engine holds for a single submitting instance.
const DefaultMaxPendingResultsPerInstance store.OccurrenceCount = 256

// maxIntervalMillis is the largest fixed interval a schedule may state
// without overflowing a time.Duration.
const maxIntervalMillis uint64 = math.MaxInt64 / uint64(time.Millisecond)

// maxInstanceLabels caps how many routing labels one live connection holds
// at once. It mirrors the wire cap on a single label list, so a connection
// cannot grow past what one registration is allowed to announce.
const maxInstanceLabels = store.LabelCount(protocol.DefaultMaxLabels)

// Recorded reasons. Each one is written onto the execution or attempt it
// explains, so an operator reads why a record stopped where it did.
const (
	waitReasonNoExecutor    = "no executor of required type connected"
	waitReasonNoCompatible  = "no compatible executor for required function version"
	waitReasonNoLabeled     = "no connected executor announces the pinned routing label"
	cancelReasonJobDisabled = "job disabled"
	cancelReasonJobReplaced = "job definition replaced"
	skipReasonOverlap       = "previous occurrence still running"
	lostReasonDisconnect    = "executor disconnected before result"
	lostReasonLease         = "execution lease expired"
	lostReasonRestart       = "scheduler restarted during dispatch"
	dispatchReasonQueueFull = "executor outgoing queue full"
	rejectReasonUnknown     = "executor rejected execution"
	failureReasonUnknown    = "function failed without error detail"
)

// Refusal reasons answered to a malformed job upsert.
const (
	upsertReasonZeroJobUUID  = "job uuid is zero"
	upsertReasonEmptyKey     = "job key is empty"
	upsertReasonEmptyType    = "executor type is empty"
	upsertReasonEmptyName    = "function name is empty"
	upsertReasonZeroInterval = "interval must not be zero"
	upsertReasonWideInterval = "interval is out of range"
	upsertReasonUnknownKind  = "unknown schedule kind"
)

const (
	// notifyBuffer is the depth of the timing-loop wakeup channel. One slot
	// is enough because a pending wakeup already covers every notification
	// raised before the loop runs again.
	notifyBuffer = 1

	// executionTimeoutMillisNone leaves an execution request without a
	// per-execution timeout, so the lease alone bounds it.
	executionTimeoutMillisNone = 0

	// firstAttemptNumber is the ordinal of the first attempt of an
	// execution and the offset between consumed attempts and the attempt
	// budget.
	firstAttemptNumber = 1
)

// logFieldInstanceID names the executor connection a log line is about.
const logFieldInstanceID = "instance_id"

const logTag = "[SCHEDULERENGINE]"
