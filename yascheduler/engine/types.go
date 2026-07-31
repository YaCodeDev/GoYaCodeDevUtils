package engine

import (
	"context"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// Clock is the time source an engine reads. Replacing it drives scheduling
// deterministically, so a test never waits on wall-clock time.
type Clock func() time.Time

// Engine is the scheduling surface a transport calls into. Start, Pause and
// Stop are the whole lifecycle; every Handle method answers one message
// arriving on an authenticated connection, so the instance identity comes
// from the connection and never from the payload.
type Engine interface {
	// Start runs startup recovery and launches the timing and reconcile
	// loops. Cancelling the given context stops both loops.
	Start(ctx context.Context)

	// Pause stops admitting new dispatch work while leaving in-flight
	// executions and their loops alone.
	Pause()

	// Stop pauses the engine, cancels its loops, and waits for them to
	// finish or for the given context to expire.
	Stop(ctx context.Context)

	// Notify wakes the timing loop, so a change that made work due is acted
	// on without waiting for the next scheduled wakeup.
	Notify()

	// SetClock replaces the engine time source.
	SetClock(now Clock)

	// Snapshot reads the engine counters by their stable metric names.
	Snapshot() map[string]uint64

	// HandleJobUpsert stores or replaces one job definition on behalf of
	// the given submitter and answers the acknowledgement to send back.
	HandleJobUpsert(
		ctx context.Context,
		instanceID protocol.InstanceID,
		upsert *protocol.JobUpsert,
	) *protocol.JobUpsertAck

	// HandleExecAccept records whether an executor admitted a dispatched
	// attempt.
	HandleExecAccept(
		ctx context.Context,
		instanceID protocol.InstanceID,
		accept *protocol.ExecAccept,
	)

	// HandleExecResult settles an accepted attempt from its terminal
	// outcome.
	HandleExecResult(
		ctx context.Context,
		instanceID protocol.InstanceID,
		result *protocol.ExecResult,
	)

	// HandleDisconnect abandons every open attempt of a departed executor
	// so its work is redispatched.
	HandleDisconnect(ctx context.Context, instanceID protocol.InstanceID)

	// HandleHeartbeat renews the lease of every open attempt on a live
	// executor.
	HandleHeartbeat(ctx context.Context, instanceID protocol.InstanceID)
}
