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
	pipelinedPairs           = 16
	pipelinedCapacity uint32 = 32
)

// startCancelClient runs a client whose capacity is raised above the
// hostile default, so every request of a batch is admitted instead of being
// refused for capacity and never reaching the cancel bookkeeping.
func startCancelClient(
	t *testing.T,
	address string,
	registry *yascheduler.Registry,
	capacity uint32,
) *hostileClient {
	t.Helper()

	config := hostileClientConfig(address)
	config.Capacity = capacity

	client, err := yascheduler.New(config, registry, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = client.Run(ctx)
	}()

	running := &hostileClient{client: client, cancel: cancel, done: done}

	t.Cleanup(func() { running.stop(t) })

	return running
}

// TestClientCancelsExecutionPipelinedBehindItsRequest proves a Cancel that
// shares a read buffer with the ExecRequest it cancels still reaches the
// running function. A scheduler that disables a job or skips an overlapping
// occurrence right after dispatch writes both frames in one connection
// write, so the client must have the cancel registered before it decodes
// the next frame rather than from inside the execution goroutine.
func TestClientCancelsExecutionPipelinedBehindItsRequest(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, pipelinedPairs)
	cancelled := make(chan struct{}, pipelinedPairs)
	registry := hostileRegistry(t, started, cancelled)

	args, encodeErr := encodeHostileArgs()
	if encodeErr != nil {
		t.Fatalf("args should encode: %v", encodeErr)
	}

	batch := pipelinedBatch(t, args)

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

		if _, err := conn.Write(batch); err != nil {
			t.Errorf("batch write failed: %v", err)

			return
		}

		time.Sleep(hostileCancelWait)
	})

	startCancelClient(t, server.addr(), registry, pipelinedCapacity)

	awaitCancels(t, cancelled, pipelinedPairs)
}

// pipelinedBatch renders alternating ExecRequest and Cancel frames into one
// buffer, so a single connection write delivers every cancel in the same
// read the client uses to decode its request.
func pipelinedBatch(t *testing.T, args []byte) []byte {
	t.Helper()

	var batch []byte

	for pair := range pipelinedPairs {
		executionID := hostileExecutionID + protocol.ExecutionID(pair)

		request, err := protocol.EncodeFrame(1, &protocol.ExecRequest{
			ExecutionID: executionID,
			AttemptID:   hostileFirstAttempt,
			Function:    protocol.FunctionSpec{Name: hostileFunction},
			Args:        args,
		}, protocol.Limits{})
		if err != nil {
			t.Fatalf("exec request should encode: %v", err)
		}

		cancel, err := protocol.EncodeFrame(2, &protocol.Cancel{
			ExecutionID: executionID,
			AttemptID:   hostileFirstAttempt,
			Reason:      "pipelined cancel",
		}, protocol.Limits{})
		if err != nil {
			t.Fatalf("cancel should encode: %v", err)
		}

		batch = append(batch, request...)
		batch = append(batch, cancel...)
	}

	return batch
}
