package yascheduler

import (
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	internalAddress                            = "127.0.0.1:1"
	internalExecutorType protocol.ExecutorType = "internal-test"

	internalCapacity  = 16
	internalQueueSize = 1024
	internalDrainWait = 2 * time.Second

	configuredInterval   = time.Second
	ceilingInterval      = 10 * time.Second
	assignedInterval     = 200 * time.Millisecond
	tinyInterval         = 5 * time.Millisecond
	overflowInterval     = 100 * 365 * 24 * time.Hour
	assignedBelowFloor   = uint32(10)
	assignedAboveCeiling = uint32(60_000)
	assignedMillis       = uint32(200)
	assignedUnset        = uint32(0)
)

func newInternalClient(
	t *testing.T,
	registry *Registry,
	heartbeatInterval time.Duration,
) *Client {
	t.Helper()

	client, err := New(&Config{
		Address:           internalAddress,
		ExecutorType:      internalExecutorType,
		Capacity:          internalCapacity,
		HeartbeatInterval: heartbeatInterval,
		DrainTimeout:      internalDrainWait,
		OutgoingQueueSize: internalQueueSize,
	}, registry, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	return client
}

// TestResolveHeartbeatIntervalStaysInBounds pins the assigned-cadence clamp
// to the range the package documents. The ceiling is derived from the
// configured cadence, so an extreme configured value must saturate instead
// of overflowing into a negative duration that panics time.NewTicker.
func TestResolveHeartbeatIntervalStaysInBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		configured time.Duration
		assigned   uint32
		want       time.Duration
	}{
		{
			name:       "unassigned cadence falls back to the configured one",
			configured: configuredInterval,
			assigned:   assignedUnset,
			want:       configuredInterval,
		},
		{
			name:       "cadence below the floor clamps to the minimum",
			configured: configuredInterval,
			assigned:   assignedBelowFloor,
			want:       MinHeartbeatInterval,
		},
		{
			name:       "cadence above the ceiling clamps to the factor bound",
			configured: configuredInterval,
			assigned:   assignedAboveCeiling,
			want:       ceilingInterval,
		},
		{
			name:       "overflowing configured cadence keeps the assigned one",
			configured: overflowInterval,
			assigned:   assignedMillis,
			want:       assignedInterval,
		},
		{
			name:       "tiny configured cadence still honours the floor",
			configured: tinyInterval,
			assigned:   assignedMillis,
			want:       MinHeartbeatInterval,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			client := newInternalClient(t, NewRegistry(), testCase.configured)

			got := client.resolveHeartbeatInterval(testCase.assigned)
			if got != testCase.want {
				t.Fatalf("interval = %s, want %s", got, testCase.want)
			}

			if got < MinHeartbeatInterval || got > MaxHeartbeatInterval {
				t.Fatalf(
					"interval %s escaped [%s, %s]",
					got,
					MinHeartbeatInterval,
					MaxHeartbeatInterval,
				)
			}
		})
	}
}
