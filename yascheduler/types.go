package yascheduler

import (
	"net/http"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/google/uuid"
)

// Config configures one executor client. Address and ExecutorType are
// required; every other field falls back to a package default. The zero
// values therefore configure a working client once the two required
// fields are set.
type Config struct {
	// Address is the host:port of the yascheduler service.
	Address string

	// ExecutorType names what kind of service this process is.
	ExecutorType protocol.ExecutorType

	// InstanceID identifies this process. Leave empty to generate one at
	// client construction; it then stays stable across every reconnect
	// of this process and is regenerated only when the process restarts.
	InstanceID protocol.InstanceID

	// Capacity bounds concurrent executions on this executor.
	Capacity uint32

	// HeartbeatInterval is the heartbeat cadence proposed to the
	// scheduler; the scheduler's registration acknowledgement wins when
	// it assigns a different one.
	HeartbeatInterval time.Duration

	// DialTimeout bounds one TCP connect attempt.
	DialTimeout time.Duration

	// WriteTimeout bounds one frame write.
	WriteTimeout time.Duration

	// DrainTimeout bounds how long a stopping client waits for running
	// functions before cancelling their contexts.
	DrainTimeout time.Duration

	// ReconnectInitialInterval seeds the reconnect backoff.
	ReconnectInitialInterval time.Duration

	// ReconnectMaxInterval caps the reconnect backoff.
	ReconnectMaxInterval time.Duration

	// OutgoingQueueSize bounds the per-connection outgoing frame queue.
	OutgoingQueueSize int

	// DefaultBackfill is this library instance's backfill default. A job
	// whose BackfillSpec.Mode is BackfillModeInherit is stamped with
	// this mode at upsert time; when this is also BackfillModeInherit
	// the scheduler default (enabled) applies. Precedence, highest
	// first: job spec, library instance, scheduler default.
	DefaultBackfill protocol.BackfillMode

	// Limits bounds every wire value this client accepts and sends.
	Limits protocol.Limits
}

// normalized validates the required fields and returns a copy with every
// zero field filled with its package default, generating an InstanceID
// when none is given.
func (c *Config) normalized() (Config, yaerrors.Error) {
	normalized := *c
	if normalized.Address == "" {
		return normalized, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyAddress,
			logTag+" config",
		)
	}

	if normalized.ExecutorType == "" {
		return normalized, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyExecutorType,
			logTag+" config",
		)
	}

	if normalized.InstanceID == "" {
		normalized.InstanceID = protocol.InstanceID(uuid.NewString())
	}

	if normalized.Capacity == 0 {
		normalized.Capacity = DefaultCapacity
	}

	if normalized.HeartbeatInterval <= 0 {
		normalized.HeartbeatInterval = DefaultHeartbeatInterval
	}

	if normalized.DialTimeout <= 0 {
		normalized.DialTimeout = DefaultDialTimeout
	}

	if normalized.WriteTimeout <= 0 {
		normalized.WriteTimeout = DefaultWriteTimeout
	}

	if normalized.DrainTimeout <= 0 {
		normalized.DrainTimeout = DefaultDrainTimeout
	}

	if normalized.ReconnectInitialInterval <= 0 {
		normalized.ReconnectInitialInterval = DefaultReconnectInitialInterval
	}

	if normalized.ReconnectMaxInterval <= 0 {
		normalized.ReconnectMaxInterval = DefaultReconnectMaxInterval
	}

	if normalized.OutgoingQueueSize <= 0 {
		normalized.OutgoingQueueSize = DefaultOutgoingQueueSize
	}

	return normalized, nil
}

// JobSpec describes one job this client asks the scheduler to store.
type JobSpec struct {
	// Key is the client-chosen stable job key; upserts with the same
	// key address the same job.
	Key string

	// ExecutorType selects which executor pool runs the job. Empty
	// defaults to this client's own executor type.
	ExecutorType protocol.ExecutorType

	// Function identifies the target function. Empty signature fields
	// are stamped from this client's local registry when the function is
	// registered here.
	Function protocol.FunctionSpec

	// Args is the function argument value; it is encoded with
	// MessagePack at upsert time.
	Args any

	// Schedule defines when the job runs.
	Schedule protocol.ScheduleSpec

	// Disabled stores the job without scheduling it.
	Disabled bool

	// Backfill configures missed-occurrence handling; the mode falls
	// back to the client's DefaultBackfill, then the scheduler default.
	Backfill protocol.BackfillSpec

	// Retry configures function-error retries; the zero value inherits
	// the scheduler default of protocol.DefaultMaxRetries retries with
	// exponential delay.
	Retry protocol.RetrySpec

	// Overlap selects what happens when an occurrence becomes due while
	// a previous one still runs; the zero value inherits the scheduler
	// default of allowing overlap.
	Overlap protocol.OverlapPolicy

	// Pin constrains which executors may run the job; an empty label pins
	// nothing. A strict pin waits for an executor announcing the label; a
	// preferred pin widens back to the whole pool once the label has no
	// taker.
	Pin protocol.PinSpec
}
