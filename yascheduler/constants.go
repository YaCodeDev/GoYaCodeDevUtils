package yascheduler

import "time"

const (
	// DefaultHeartbeatInterval is the heartbeat cadence used when the
	// scheduler does not assign one in its registration acknowledgement.
	DefaultHeartbeatInterval = 5 * time.Second

	// MinHeartbeatInterval is the shortest cadence a client accepts from
	// the scheduler, so a hostile assignment cannot turn the connection
	// into a heartbeat flood.
	MinHeartbeatInterval = 100 * time.Millisecond

	// MaxHeartbeatInterval is the longest cadence a client accepts from
	// the scheduler. The read deadline is a multiple of the cadence, so
	// without this bound an assigned interval could stretch the deadline
	// far enough to disable dead- and half-open-connection detection.
	MaxHeartbeatInterval = 5 * time.Minute

	// MaxHeartbeatFactor bounds the assigned cadence relative to the
	// client's own configured interval. A client that asks for a fast
	// heartbeat is stating how long it tolerates silence, so a scheduler
	// may stretch that by this factor and no further.
	MaxHeartbeatFactor = 10

	// DefaultDialTimeout bounds one TCP connect attempt.
	DefaultDialTimeout = 10 * time.Second

	// DefaultWriteTimeout bounds one frame write.
	DefaultWriteTimeout = 10 * time.Second

	// DefaultDrainTimeout bounds how long a stopping client waits for
	// running functions before cancelling their contexts.
	DefaultDrainTimeout = 30 * time.Second

	// DefaultReconnectInitialInterval seeds the reconnect backoff.
	DefaultReconnectInitialInterval = 500 * time.Millisecond

	// DefaultReconnectMaxInterval caps the reconnect backoff.
	DefaultReconnectMaxInterval = 30 * time.Second

	// DefaultReconnectMultiplier grows the reconnect backoff.
	DefaultReconnectMultiplier = 2.0

	// DefaultOutgoingQueueSize bounds the per-connection outgoing frame
	// queue.
	DefaultOutgoingQueueSize = 256

	// DefaultCapacity bounds concurrent executions when the
	// configuration leaves Capacity zero.
	DefaultCapacity uint32 = 64

	// readDeadlineMultiplier scales the heartbeat interval into the read
	// deadline that detects dead and half-open connections.
	readDeadlineMultiplier = 3

	// jitterMinFactor is the lower bound of the random reconnect jitter
	// factor; the delay is scaled into [jitterMinFactor, 1.0].
	jitterMinFactor = 0.5
)

const logTag = "[SCHEDULER]"
