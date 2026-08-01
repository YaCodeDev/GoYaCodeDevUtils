package yascheduler

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	registryDeliverTimeout = 2 * time.Second

	registryExecutionID protocol.ExecutionID = 9
)

var registryJobUUID = protocol.JobUUID{
	0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x47, 0x28,
	0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30,
}

func newTestResultRegistry() *resultRegistry {
	return &resultRegistry{waiters: make(map[protocol.JobUUID]chan *Result)}
}

func (r *resultRegistry) waiterCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.waiters)
}

// TestResultRegistryReleasesEntries pins the leak contract on the waiter
// map: the channel itself is collectable once unreferenced, but the map
// entry is not, so every Submission exit path must remove it.
func TestResultRegistryReleasesEntries(t *testing.T) {
	t.Parallel()

	t.Run("when the submission closes / then the entry is removed", func(t *testing.T) {
		t.Parallel()

		registry := newTestResultRegistry()

		submission := registry.open(registryJobUUID, protocol.ResultModeDeliver)

		if registry.waiterCount() != 1 {
			t.Fatalf("waiters = %d after open, want 1", registry.waiterCount())
		}

		submission.Close()

		if registry.waiterCount() != 0 {
			t.Fatalf("waiters = %d after Close, want 0", registry.waiterCount())
		}

		submission.Close()

		if registry.waiterCount() != 0 {
			t.Fatalf("waiters = %d after second Close, want 0", registry.waiterCount())
		}
	})

	t.Run("when an await consumes the result / then the entry is removed", func(t *testing.T) {
		t.Parallel()

		registry := newTestResultRegistry()

		submission := registry.open(registryJobUUID, protocol.ResultModeDeliver)

		if !registry.deliver(&Result{JobUUID: registryJobUUID, Success: true}) {
			t.Fatal("delivery to a registered waiter was refused")
		}

		ctx, cancel := context.WithTimeout(context.Background(), registryDeliverTimeout)
		defer cancel()

		if _, err := submission.Await(ctx); err != nil {
			t.Fatalf("Await failed: %v", err)
		}

		if registry.waiterCount() != 0 {
			t.Fatalf("waiters = %d after consumption, want 0", registry.waiterCount())
		}
	})

	t.Run("when an await is cancelled / then the entry is removed", func(t *testing.T) {
		t.Parallel()

		registry := newTestResultRegistry()

		submission := registry.open(registryJobUUID, protocol.ResultModeDeliver)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if _, err := submission.Await(ctx); err == nil {
			t.Fatal("Await returned nil on a cancelled context")
		}

		if registry.waiterCount() != 0 {
			t.Fatalf("waiters = %d after cancelled Await, want 0", registry.waiterCount())
		}
	})

	t.Run("when the mode is ignore / then no entry is registered", func(t *testing.T) {
		t.Parallel()

		registry := newTestResultRegistry()

		submission := registry.open(registryJobUUID, protocol.ResultModeIgnore)

		if registry.waiterCount() != 0 {
			t.Fatalf("waiters = %d after ignore-mode open, want 0", registry.waiterCount())
		}

		submission.Close()
	})
}

// TestResultRegistryDeliverNeverBlocks pins the duplicate contract:
// delivery is at-least-once, so a duplicate finds the one-slot buffer full
// and is discarded, while a blocking send would wedge the delivery path on
// an abandoned wait.
func TestResultRegistryDeliverNeverBlocks(t *testing.T) {
	t.Parallel()

	registry := newTestResultRegistry()

	submission := registry.open(registryJobUUID, protocol.ResultModeDeliver)
	defer submission.Close()

	if !registry.deliver(&Result{
		JobUUID:     registryJobUUID,
		ExecutionID: registryExecutionID,
	}) {
		t.Fatal("first delivery was refused")
	}

	duplicate := make(chan bool, 1)

	go func() {
		duplicate <- registry.deliver(&Result{
			JobUUID:     registryJobUUID,
			ExecutionID: registryExecutionID + 1,
		})
	}()

	select {
	case accepted := <-duplicate:
		if !accepted {
			t.Fatal("duplicate delivery reported no waiter while one is registered")
		}
	case <-time.After(registryDeliverTimeout):
		t.Fatal("duplicate delivery blocked on a full waiter buffer")
	}

	if registry.deliver(&Result{JobUUID: protocol.JobUUID{}, ExecutionID: 1}) {
		t.Fatal("delivery for an unknown job reported a waiter")
	}
}

// TestClientDetachKeepsResultWaiters pins the lifecycle split on
// connection teardown: correlation-scoped pending replies die with the
// connection, result waiters are keyed by job UUID and survive it.
func TestClientDetachKeepsResultWaiters(t *testing.T) {
	t.Parallel()

	client := newInternalClient(t, NewRegistry(), configuredInterval)

	submission := client.results.open(registryJobUUID, protocol.ResultModeDeliver)
	defer submission.Close()

	client.attachConnection(make(chan []byte, 1))

	waiter, err := client.registerPending(protocol.CorrelationID(1))
	if err != nil {
		t.Fatalf("registerPending failed: %v", err)
	}

	client.detachConnection()

	select {
	case _, open := <-waiter:
		if open {
			t.Fatal("pending reply waiter received a value instead of closing")
		}
	default:
		t.Fatal("pending reply waiter survived the detach")
	}

	if client.results.waiterCount() != 1 {
		t.Fatalf(
			"result waiters = %d after detach, want 1: waiters are keyed by "+
				"job UUID and must survive reconnects",
			client.results.waiterCount(),
		)
	}
}
