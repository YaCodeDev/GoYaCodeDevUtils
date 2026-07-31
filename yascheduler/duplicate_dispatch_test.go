package yascheduler_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	duplicateTransient protocol.FunctionName = "duplicate-transient"
	duplicateBlocking  protocol.FunctionName = "duplicate-blocking"
	duplicateCapacity  uint32                = 4

	// duplicateSettleWait lets the finished copy retire its cancel entry
	// while staying well inside the client's read deadline, so the Cancel
	// that follows lands on the same connection.
	duplicateSettleWait = 50 * time.Millisecond
)

// TestClientCancelsDuplicateDispatchOfOneAttempt proves a Cancel still
// reaches a running execution after an identically identified copy of it
// finished. A scheduler that redispatches the same execution and attempt
// pair puts two invocations on one executor, and keying the cancel
// bookkeeping by that pair lets the first copy's cleanup strand the second.
func TestClientCancelsDuplicateDispatchOfOneAttempt(t *testing.T) {
	t.Parallel()

	transientStarted := make(chan struct{}, 1)
	blockingStarted := make(chan struct{}, 1)
	blockingCancelled := make(chan struct{}, 1)
	release := make(chan struct{})

	registry := duplicateRegistry(
		t,
		transientStarted,
		release,
		blockingStarted,
		blockingCancelled,
	)

	args, encodeErr := encodeHostileArgs()
	if encodeErr != nil {
		t.Fatalf("args should encode: %v", encodeErr)
	}

	server := startHostileServer(t, func(t *testing.T, conn net.Conn, index int64) {
		readRegisterAndAck(t, conn, testHeartbeatMilli)

		if index != 1 {
			time.Sleep(hostileSettleWait)

			return
		}

		if err := conn.SetDeadline(time.Time{}); err != nil {
			t.Errorf("clear deadline failed: %v", err)

			return
		}

		for _, name := range []protocol.FunctionName{
			duplicateTransient,
			duplicateBlocking,
		} {
			if err := protocol.WriteFrame(conn, 1, &protocol.ExecRequest{
				ExecutionID: hostileExecutionID,
				AttemptID:   hostileFirstAttempt,
				Function:    protocol.FunctionSpec{Name: name},
				Args:        args,
			}, protocol.Limits{}); err != nil {
				t.Errorf("exec request write failed: %v", err)

				return
			}

			awaitSignal(t, transientStarted, blockingStarted, name)
		}

		close(release)

		time.Sleep(duplicateSettleWait)

		if err := protocol.WriteFrame(conn, 2, &protocol.Cancel{
			ExecutionID: hostileExecutionID,
			AttemptID:   hostileFirstAttempt,
			Reason:      "scheduler cancelled",
		}, protocol.Limits{}); err != nil {
			t.Errorf("cancel write failed: %v", err)

			return
		}

		time.Sleep(hostileCancelWait)
	})

	startCancelClient(t, server.addr(), registry, duplicateCapacity)

	select {
	case <-blockingCancelled:
	case <-time.After(hostileCancelWait):
		t.Fatal(
			"the surviving copy never observed the cancel: a finished copy of " +
				"one execution must not strand the copy still running",
		)
	}
}

// awaitSignal blocks until the dispatched copy reported that it started.
func awaitSignal(
	t *testing.T,
	transientStarted chan struct{},
	blockingStarted chan struct{},
	name protocol.FunctionName,
) {
	t.Helper()

	signal := transientStarted
	if name == duplicateBlocking {
		signal = blockingStarted
	}

	select {
	case <-signal:
	case <-time.After(hostileCancelWait):
		t.Errorf("execution %q never started", name)
	}
}

// duplicateRegistry holds one copy that finishes on demand and one that
// runs until its context ends, so a test can retire the first copy of an
// execution while the second is still running.
func duplicateRegistry(
	t *testing.T,
	transientStarted chan struct{},
	release chan struct{},
	blockingStarted chan struct{},
	blockingCancelled chan struct{},
) *yascheduler.Registry {
	t.Helper()

	registry := yascheduler.NewRegistry()

	if err := yascheduler.RegisterFunction(
		registry,
		duplicateTransient,
		"",
		func(_ context.Context, value int64) (int64, error) {
			transientStarted <- struct{}{}
			<-release

			return value, nil
		},
	); err != nil {
		t.Fatalf("transient function should register: %v", err)
	}

	if err := yascheduler.RegisterFunction(
		registry,
		duplicateBlocking,
		"",
		func(ctx context.Context, value int64) (int64, error) {
			blockingStarted <- struct{}{}
			<-ctx.Done()
			blockingCancelled <- struct{}{}

			return value, errHostileCancelled
		},
	); err != nil {
		t.Fatalf("blocking function should register: %v", err)
	}

	return registry
}
