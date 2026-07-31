package engine

import "sync/atomic"

// Metrics counts what one engine did. Every counter is monotonic and safe
// to read from any goroutine.
type Metrics struct {
	Dispatches        atomic.Uint64
	DispatchFailures  atomic.Uint64
	FunctionSuccesses atomic.Uint64
	FunctionFailures  atomic.Uint64
	FunctionRetries   atomic.Uint64
	InfraRedispatches atomic.Uint64
	WaitingExecutor   atomic.Uint64
	WaitingCompatible atomic.Uint64
	WaitingCapacity   atomic.Uint64
	WaitingLabel      atomic.Uint64
	SkippedOverlaps   atomic.Uint64
	BackfillCreated   atomic.Uint64
	BackfillSkipped   atomic.Uint64
	StaleMessages     atomic.Uint64
	ProtocolErrors    atomic.Uint64

	// LabelPinFallbacks counts dispatches that widened a preferred pin back
	// to the whole pool because the pinned label had no taker.
	LabelPinFallbacks atomic.Uint64

	// LabelUpdatesRejected counts label updates refused by admission, each
	// of which leaves the connection's label set untouched.
	LabelUpdatesRejected atomic.Uint64

	// LabelWithdrawnInFlight counts label withdrawals that landed while the
	// withdrawing connection still owed results. Those attempts are left to
	// finish: labels bind at dispatch.
	LabelWithdrawnInFlight atomic.Uint64
}

// Snapshot reads every counter into a map keyed by its stable metric name.
func (m *Metrics) Snapshot() (snapshot map[string]uint64) {
	return map[string]uint64{
		"dispatches":         m.Dispatches.Load(),
		"dispatch_failures":  m.DispatchFailures.Load(),
		"function_successes": m.FunctionSuccesses.Load(),
		"function_failures":  m.FunctionFailures.Load(),
		"function_retries":   m.FunctionRetries.Load(),
		"infra_redispatches": m.InfraRedispatches.Load(),
		"waiting_executor":   m.WaitingExecutor.Load(),
		"waiting_compatible": m.WaitingCompatible.Load(),
		"waiting_capacity":   m.WaitingCapacity.Load(),
		"waiting_label":      m.WaitingLabel.Load(),
		"skipped_overlaps":   m.SkippedOverlaps.Load(),
		"backfill_created":   m.BackfillCreated.Load(),
		"backfill_skipped":   m.BackfillSkipped.Load(),
		"stale_messages":     m.StaleMessages.Load(),
		"protocol_errors":    m.ProtocolErrors.Load(),

		"label_pin_fallbacks":       m.LabelPinFallbacks.Load(),
		"label_updates_rejected":    m.LabelUpdatesRejected.Load(),
		"label_withdrawn_in_flight": m.LabelWithdrawnInFlight.Load(),
	}
}

func metricsFields(snapshot map[string]uint64) (fields map[string]any) {
	fields = make(map[string]any, len(snapshot))

	for key, value := range snapshot {
		fields[key] = value
	}

	return fields
}
