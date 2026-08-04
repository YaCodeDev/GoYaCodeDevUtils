package engine

import (
	"testing"
	"time"
)

func TestConfigNormalizedDefaultsExecutionRetention(t *testing.T) {
	t.Parallel()

	normalized := (&Config{}).normalized()

	if normalized.ExecutionRetention != DefaultExecutionRetention {
		t.Errorf(
			"a zero execution retention should fall back to the default: got %v, want %v",
			normalized.ExecutionRetention,
			DefaultExecutionRetention,
		)
	}
}

func TestConfigNormalizedClampsExecutionRetentionToBackfillMaxAge(t *testing.T) {
	t.Parallel()

	const (
		shortRetention = time.Hour
		longBackfill   = 3 * time.Hour
	)

	normalized := (&Config{
		ExecutionRetention: shortRetention,
		BackfillMaxAge:     longBackfill,
	}).normalized()

	if normalized.ExecutionRetention != longBackfill {
		t.Errorf(
			"execution retention must never undercut the backfill window: got %v, want %v",
			normalized.ExecutionRetention,
			longBackfill,
		)
	}
}
