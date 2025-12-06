package yabackoff_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabackoff"
)

func TestEmptySafety_Works(t *testing.T) {
	t.Parallel()

	exp := yabackoff.Exponential{}
	got := exp.Next()

	expected := yabackoff.NewExponential(
		yabackoff.DefaultInitialInterval,
		yabackoff.DefaultMultiplier,
		yabackoff.DefaultMaxInterval,
		0,
	)
	want := expected.Next()

	assert.Equal(t, want, got)
}

func TestNext_Works(t *testing.T) {
	t.Parallel()

	start := 500 * time.Millisecond
	multiplier := 1.5
	maxInterval := 10 * time.Second

	backoff := yabackoff.NewExponential(start, multiplier, maxInterval, 0)

	expected := []time.Duration{start}

	for {
		last := expected[len(expected)-1]

		next := min(time.Duration(float64(last)*multiplier), maxInterval)

		expected = append(expected, next)

		if next == maxInterval {
			break
		}
	}

	for i, want := range expected {
		got := backoff.Next()

		assert.Equal(t, want, got, "mismatch at step %d", i)
	}
}

func TestReset_Works(t *testing.T) {
	t.Parallel()

	start := time.Second

	b := yabackoff.NewExponential(start, 2.0, 10*time.Second, 0)

	b.Next()
	b.Next()

	b.Reset()

	assert.Equal(t, start, b.Current())
}

func TestMaxIntervalIsRespected(t *testing.T) {
	t.Parallel()

	maxInterval := 5 * time.Second

	backoff := yabackoff.NewExponential(2*time.Second, 10, maxInterval, 0)

	backoff.Next()

	backoff.Next()

	assert.Equal(t, maxInterval, backoff.Current())
}

func TestWaitDoesSleep(t *testing.T) {
	t.Parallel()

	start := 100 * time.Millisecond
	backoff := yabackoff.NewExponential(start, 1.0, time.Second, 0)

	startWaiting := time.Now()

	backoff.Wait()

	elapsed := time.Since(startWaiting)

	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
}

func TestResetAfter(t *testing.T) {
	t.Parallel()

	start := 100 * time.Millisecond
	resetAfter := 50 * time.Millisecond

	backoff := yabackoff.NewExponential(start, 2.0, time.Second, resetAfter)

	backoff.Next()

	expected := backoff.Current()

	backoff.Next()

	time.Sleep(resetAfter + backoff.Current())

	backoff.Next()

	assert.Equal(t, expected, backoff.Current())
}
