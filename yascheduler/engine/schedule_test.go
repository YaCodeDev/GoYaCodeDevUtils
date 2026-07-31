package engine

import (
	"math"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	anchorUnixSeconds = 1_700_000_000
	stepInterval      = time.Minute
	noOccurrenceCap   = store.OccurrenceCount(0)
	noAgeCap          = time.Duration(0)
	retryInitialCfg   = time.Second
	retryMaxCfg       = time.Minute
)

var anchorTime = time.Unix(anchorUnixSeconds, 0).UTC()

func retryConfig() *Config {
	return &Config{
		RetryInitialDelay: retryInitialCfg,
		RetryMaxDelay:     retryMaxCfg,
	}
}

func oneShotSpec(start time.Time) protocol.ScheduleSpec {
	return protocol.ScheduleSpec{
		Kind:          protocol.ScheduleKindOneShot,
		StartUnixNano: start.UnixNano(),
	}
}

func intervalSpec(start time.Time, interval time.Duration) protocol.ScheduleSpec {
	return protocol.ScheduleSpec{
		Kind:           protocol.ScheduleKindFixedInterval,
		StartUnixNano:  start.UnixNano(),
		IntervalMillis: uint64(interval / time.Millisecond),
	}
}

func TestNextOccurrence(t *testing.T) {
	t.Parallel()

	t.Run(
		"when a one-shot start is in the future / then it is the next occurrence",
		func(t *testing.T) {
			t.Parallel()

			start := anchorTime.Add(time.Hour)

			occurrence, exists := nextOccurrence(oneShotSpec(start), anchorTime)
			if !exists {
				t.Fatal("a future one-shot should have a next occurrence")
			}

			if !occurrence.Equal(start) {
				t.Errorf(
					"the next occurrence should be the start: got %v, want %v",
					occurrence,
					start,
				)
			}
		},
	)

	t.Run(
		"when a one-shot start has passed or is now / then no occurrence remains",
		func(t *testing.T) {
			t.Parallel()

			if _, exists := nextOccurrence(
				oneShotSpec(anchorTime.Add(-time.Hour)),
				anchorTime,
			); exists {
				t.Error("a past one-shot should have no next occurrence")
			}

			if _, exists := nextOccurrence(oneShotSpec(anchorTime), anchorTime); exists {
				t.Error("a one-shot due exactly now should have no next occurrence")
			}
		},
	)

	t.Run(
		"when an interval start is in the future / then the start itself is next",
		func(t *testing.T) {
			t.Parallel()

			start := anchorTime.Add(time.Hour)

			occurrence, exists := nextOccurrence(intervalSpec(start, stepInterval), anchorTime)
			if !exists {
				t.Fatal("a future interval schedule should have a next occurrence")
			}

			if !occurrence.Equal(start) {
				t.Errorf(
					"the next occurrence should be the start: got %v, want %v",
					occurrence,
					start,
				)
			}
		},
	)

	t.Run(
		"when the reference sits on a boundary / then the strictly next boundary is chosen",
		func(t *testing.T) {
			t.Parallel()

			spec := intervalSpec(anchorTime, stepInterval)

			fromStart, exists := nextOccurrence(spec, anchorTime)
			if !exists {
				t.Fatal("an interval schedule should always have a next occurrence")
			}

			if want := anchorTime.Add(stepInterval); !fromStart.Equal(want) {
				t.Errorf(
					"from the start boundary the next should be one step: got %v, want %v",
					fromStart,
					want,
				)
			}

			fromBoundary, exists := nextOccurrence(spec, anchorTime.Add(stepInterval))
			if !exists {
				t.Fatal("an interval schedule should always have a next occurrence")
			}

			if want := anchorTime.Add(2 * stepInterval); !fromBoundary.Equal(want) {
				t.Errorf(
					"from a boundary the next should be one step later: got %v, want %v",
					fromBoundary,
					want,
				)
			}
		},
	)

	t.Run(
		"when the reference sits inside a period / then the next boundary is exact",
		func(t *testing.T) {
			t.Parallel()

			const midPeriodOffset = 90 * time.Second

			spec := intervalSpec(anchorTime, stepInterval)

			occurrence, exists := nextOccurrence(spec, anchorTime.Add(midPeriodOffset))
			if !exists {
				t.Fatal("an interval schedule should always have a next occurrence")
			}

			if want := anchorTime.Add(2 * stepInterval); !occurrence.Equal(want) {
				t.Errorf(
					"the next occurrence should land on the grid: got %v, want %v",
					occurrence,
					want,
				)
			}
		},
	)
}

func TestMissedOccurrences(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the start is in the future / then nothing is missed",
		func(t *testing.T) {
			t.Parallel()

			missed, skipped := missedOccurrences(
				intervalSpec(anchorTime.Add(time.Hour), stepInterval),
				anchorTime,
				noOccurrenceCap,
				noAgeCap,
			)

			if len(missed) != 0 || skipped != 0 {
				t.Errorf(
					"a future schedule should miss nothing: got %v skipped %d",
					missed,
					skipped,
				)
			}
		},
	)

	t.Run(
		"when a one-shot start has passed / then it is the single missed occurrence",
		func(t *testing.T) {
			t.Parallel()

			start := anchorTime.Add(-time.Minute)

			missed, skipped := missedOccurrences(
				oneShotSpec(start),
				anchorTime,
				noOccurrenceCap,
				noAgeCap,
			)

			if len(missed) != 1 || !missed[0].Equal(start) {
				t.Fatalf("a past one-shot should be missed once: got %v", missed)
			}

			if skipped != 0 {
				t.Errorf("an uncapped one-shot should skip nothing: got %d", skipped)
			}
		},
	)

	t.Run(
		"when a one-shot is older than the age cap / then it is skipped",
		func(t *testing.T) {
			t.Parallel()

			const (
				ageCap          = time.Hour
				expectedSkipped = store.OccurrenceCount(1)
			)

			missed, skipped := missedOccurrences(
				oneShotSpec(anchorTime.Add(-2*time.Hour)),
				anchorTime,
				noOccurrenceCap,
				ageCap,
			)

			if len(missed) != 0 {
				t.Errorf("an outdated one-shot should not be replayed: got %v", missed)
			}

			if skipped != expectedSkipped {
				t.Errorf("an outdated one-shot should count as skipped: got %d", skipped)
			}
		},
	)

	t.Run(
		"when interval occurrences were missed / then they return oldest first",
		func(t *testing.T) {
			t.Parallel()

			const (
				startOffset   = 9*time.Minute + 30*time.Second
				expectedCount = 10
			)

			start := anchorTime.Add(-startOffset)

			missed, skipped := missedOccurrences(
				intervalSpec(start, stepInterval),
				anchorTime,
				noOccurrenceCap,
				noAgeCap,
			)

			if len(missed) != expectedCount {
				t.Fatalf("every elapsed boundary should be missed: got %d", len(missed))
			}

			if skipped != 0 {
				t.Errorf("an uncapped backlog should skip nothing: got %d", skipped)
			}

			for index, occurrence := range missed {
				want := start.Add(time.Duration(index) * stepInterval)
				if !occurrence.Equal(want) {
					t.Errorf(
						"occurrence %d should be oldest first on the grid: got %v, want %v",
						index,
						occurrence,
						want,
					)
				}
			}
		},
	)

	t.Run(
		"when a count cap applies / then the most recent occurrences survive",
		func(t *testing.T) {
			t.Parallel()

			const (
				startOffset     = 9*time.Minute + 30*time.Second
				occurrenceCap   = store.OccurrenceCount(3)
				expectedSkipped = store.OccurrenceCount(7)
			)

			start := anchorTime.Add(-startOffset)

			missed, skipped := missedOccurrences(
				intervalSpec(start, stepInterval),
				anchorTime,
				occurrenceCap,
				noAgeCap,
			)

			if store.OccurrenceCount(len(missed)) != occurrenceCap {
				t.Fatalf("the count cap should bound the backlog: got %d", len(missed))
			}

			if skipped != expectedSkipped {
				t.Errorf("older occurrences should count as skipped: got %d", skipped)
			}

			newest := missed[len(missed)-1]
			if want := start.Add(9 * stepInterval); !newest.Equal(want) {
				t.Errorf(
					"the newest occurrence should survive the cap: got %v, want %v",
					newest,
					want,
				)
			}

			oldestKept := missed[0]
			if want := start.Add(7 * stepInterval); !oldestKept.Equal(want) {
				t.Errorf(
					"the cap should keep the most recent run: got %v, want %v",
					oldestKept,
					want,
				)
			}
		},
	)

	t.Run(
		"when an age cap applies / then older occurrences are cut",
		func(t *testing.T) {
			t.Parallel()

			const (
				startOffset     = 9*time.Minute + 30*time.Second
				ageCap          = 3*time.Minute + 30*time.Second
				expectedCount   = 4
				expectedSkipped = store.OccurrenceCount(6)
			)

			start := anchorTime.Add(-startOffset)

			missed, skipped := missedOccurrences(
				intervalSpec(start, stepInterval),
				anchorTime,
				noOccurrenceCap,
				ageCap,
			)

			if len(missed) != expectedCount {
				t.Fatalf("only occurrences inside the age cap should stay: got %d", len(missed))
			}

			if skipped != expectedSkipped {
				t.Errorf("occurrences beyond the age cap should be skipped: got %d", skipped)
			}

			if want := start.Add(6 * stepInterval); !missed[0].Equal(want) {
				t.Errorf("the boundary occurrence should be kept: got %v, want %v", missed[0], want)
			}
		},
	)

	t.Run(
		"when count and age caps combine / then the tighter cap wins",
		func(t *testing.T) {
			t.Parallel()

			const (
				startOffset     = 9*time.Minute + 30*time.Second
				ageCap          = 3*time.Minute + 30*time.Second
				occurrenceCap   = store.OccurrenceCount(2)
				expectedSkipped = store.OccurrenceCount(8)
			)

			start := anchorTime.Add(-startOffset)

			missed, skipped := missedOccurrences(
				intervalSpec(start, stepInterval),
				anchorTime,
				occurrenceCap,
				ageCap,
			)

			if store.OccurrenceCount(len(missed)) != occurrenceCap {
				t.Fatalf("the tighter cap should bound the backlog: got %d", len(missed))
			}

			if skipped != expectedSkipped {
				t.Errorf("everything beyond the tighter cap should be skipped: got %d", skipped)
			}

			if want := start.Add(8 * stepInterval); !missed[0].Equal(want) {
				t.Errorf(
					"the most recent occurrences should survive: got %v, want %v",
					missed[0],
					want,
				)
			}
		},
	)
}

func TestRetryDelay(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the policy is immediate or none / then there is no delay",
		func(t *testing.T) {
			t.Parallel()

			const consumed = store.FunctionAttempts(1)

			immediate := protocol.RetrySpec{Policy: protocol.RetryPolicyImmediate}
			if got := retryDelay(immediate, consumed, retryConfig()); got != 0 {
				t.Errorf("an immediate policy should not delay: got %v", got)
			}

			none := protocol.RetrySpec{Policy: protocol.RetryPolicyNone}
			if got := retryDelay(none, consumed, retryConfig()); got != 0 {
				t.Errorf("a disabled policy should not delay: got %v", got)
			}
		},
	)

	t.Run(
		"when the policy is fixed / then the initial delay always applies",
		func(t *testing.T) {
			t.Parallel()

			const (
				initialMillis = uint64(500)
				expectedDelay = 500 * time.Millisecond
				laterAttempt  = store.FunctionAttempts(3)
			)

			fixed := protocol.RetrySpec{
				Policy:             protocol.RetryPolicyFixed,
				InitialDelayMillis: initialMillis,
			}

			if got := retryDelay(fixed, laterAttempt, retryConfig()); got != expectedDelay {
				t.Errorf("a fixed policy should keep the initial delay: got %v", got)
			}
		},
	)

	t.Run(
		"when the policy is exponential / then delays double up to the cap",
		func(t *testing.T) {
			t.Parallel()

			const (
				initialMillis = uint64(1000)
				maxMillis     = uint64(4000)
			)

			spec := protocol.RetrySpec{
				Policy:             protocol.RetryPolicyExponential,
				InitialDelayMillis: initialMillis,
				MaxDelayMillis:     maxMillis,
				MultiplierBits:     math.Float64bits(2.0),
			}

			expected := map[store.FunctionAttempts]time.Duration{
				1: time.Second,
				2: 2 * time.Second,
				3: 4 * time.Second,
				4: 4 * time.Second,
			}

			for consumed, want := range expected {
				if got := retryDelay(spec, consumed, retryConfig()); got != want {
					t.Errorf("attempt %d should delay %v, got %v", consumed, want, got)
				}
			}
		},
	)

	t.Run(
		"when the spec inherits / then configuration defaults drive the delay",
		func(t *testing.T) {
			t.Parallel()

			const (
				firstConsumed = store.FunctionAttempts(1)
				thirdConsumed = store.FunctionAttempts(3)
				firstDelay    = time.Second
				thirdDelay    = 4 * time.Second
			)

			inherit := protocol.RetrySpec{Policy: protocol.RetryPolicyInherit}

			if got := retryDelay(inherit, firstConsumed, retryConfig()); got != firstDelay {
				t.Errorf("the first retry should use the configured initial delay: got %v", got)
			}

			if got := retryDelay(inherit, thirdConsumed, retryConfig()); got != thirdDelay {
				t.Errorf("later retries should grow exponentially from config: got %v", got)
			}
		},
	)

	t.Run(
		"when the multiplier bits are invalid / then the default multiplier applies",
		func(t *testing.T) {
			t.Parallel()

			const (
				initialMillis = uint64(1000)
				maxMillis     = uint64(60000)
				thirdConsumed = store.FunctionAttempts(3)
				expectedDelay = 4 * time.Second
			)

			zeroBits := protocol.RetrySpec{
				Policy:             protocol.RetryPolicyExponential,
				InitialDelayMillis: initialMillis,
				MaxDelayMillis:     maxMillis,
				MultiplierBits:     0,
			}

			if got := retryDelay(zeroBits, thirdConsumed, retryConfig()); got != expectedDelay {
				t.Errorf("zero multiplier bits should fall back to the default: got %v", got)
			}

			nanBits := zeroBits
			nanBits.MultiplierBits = math.Float64bits(math.NaN())

			if got := retryDelay(nanBits, thirdConsumed, retryConfig()); got != expectedDelay {
				t.Errorf("NaN multiplier bits should fall back to the default: got %v", got)
			}
		},
	)
}

func TestMaxFunctionAttempts(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the policy varies / then the attempt budget follows the retry count",
		func(t *testing.T) {
			t.Parallel()

			const (
				inheritedAttempts = store.FunctionAttempts(4)
				singleAttempt     = store.FunctionAttempts(1)
				explicitRetries   = uint32(2)
				explicitAttempts  = store.FunctionAttempts(3)
			)

			inherit := protocol.RetrySpec{Policy: protocol.RetryPolicyInherit}
			if got := maxFunctionAttempts(inherit); got != inheritedAttempts {
				t.Errorf("inheriting should budget %d attempts, got %d", inheritedAttempts, got)
			}

			none := protocol.RetrySpec{Policy: protocol.RetryPolicyNone}
			if got := maxFunctionAttempts(none); got != singleAttempt {
				t.Errorf("no retries should budget %d attempt, got %d", singleAttempt, got)
			}

			explicit := protocol.RetrySpec{
				Policy:     protocol.RetryPolicyExponential,
				MaxRetries: explicitRetries,
			}
			if got := maxFunctionAttempts(explicit); got != explicitAttempts {
				t.Errorf(
					"explicit retries should budget %d attempts, got %d",
					explicitAttempts,
					got,
				)
			}
		},
	)
}
