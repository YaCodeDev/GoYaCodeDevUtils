package engine

import (
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// Config tunes one scheduling engine. Every field falls back to a package
// default, so the zero value configures a working engine.
type Config struct {
	// Lease bounds how long a dispatched or running execution may go
	// without a heartbeat before a reconcile pass reclaims it.
	Lease time.Duration

	// MaxExecution caps how long heartbeats keep renewing the lease of one
	// attempt, so a wedged executor cannot hold an execution open forever.
	MaxExecution time.Duration

	// ReconcileInterval is the cadence of the reconcile pass that reaps
	// expired leases, requeues waiting work, and tops up occurrences.
	ReconcileInterval time.Duration

	// DispatchBatch caps how many due executions one timing-loop pass
	// dispatches.
	DispatchBatch store.BatchLimit

	// RedispatchDelay delays the requeue of an execution whose delivery
	// failed for an infrastructure reason.
	RedispatchDelay time.Duration

	// RetryInitialDelay seeds the function-error retry delay of a job that
	// states none of its own.
	RetryInitialDelay time.Duration

	// RetryMaxDelay caps the function-error retry delay of a job that
	// states none of its own.
	RetryMaxDelay time.Duration

	// DefaultBackfill is this engine's backfill default. A job whose
	// BackfillSpec.Mode is BackfillModeInherit falls back to this mode;
	// when this is also BackfillModeInherit, missed occurrences are
	// materialized. Precedence, highest first: job spec, engine default,
	// enabled.
	DefaultBackfill protocol.BackfillMode

	// BackfillMaxCount caps how many missed occurrences one job
	// materializes, whatever the job's own spec asks for.
	BackfillMaxCount store.OccurrenceCount

	// BackfillMaxAge caps how old a missed occurrence may be and still be
	// materialized, whatever the job's own spec asks for.
	BackfillMaxAge time.Duration
}

// normalized returns a copy with every zero or negative field replaced by
// its package default.
func (c *Config) normalized() (normalized Config) {
	normalized = *c

	if normalized.Lease <= 0 {
		normalized.Lease = DefaultLease
	}

	if normalized.MaxExecution <= 0 {
		normalized.MaxExecution = DefaultMaxExecution
	}

	if normalized.ReconcileInterval <= 0 {
		normalized.ReconcileInterval = DefaultReconcileInterval
	}

	if normalized.DispatchBatch <= 0 {
		normalized.DispatchBatch = DefaultDispatchBatch
	}

	if normalized.RedispatchDelay <= 0 {
		normalized.RedispatchDelay = DefaultRedispatchDelay
	}

	if normalized.RetryInitialDelay <= 0 {
		normalized.RetryInitialDelay = DefaultRetryInitialDelay
	}

	if normalized.RetryMaxDelay <= 0 {
		normalized.RetryMaxDelay = DefaultRetryMaxDelay
	}

	if normalized.BackfillMaxCount == 0 {
		normalized.BackfillMaxCount = DefaultBackfillMaxCount
	}

	if normalized.BackfillMaxAge <= 0 {
		normalized.BackfillMaxAge = DefaultBackfillMaxAge
	}

	return normalized
}
