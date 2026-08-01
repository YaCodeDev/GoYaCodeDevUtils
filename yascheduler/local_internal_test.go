package yascheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	loopbackQueueOne  = 1
	loopbackWaitShort = 2 * time.Second

	sinkExecutorType protocol.ExecutorType = "sink-check"

	upsertStubFunction protocol.FunctionName = "upsert-stub"

	upsertStubValue       int64                = 42
	upsertStubExecutionID protocol.ExecutionID = 1
)

func newTestLoopback(queueSize int) *loopback {
	return newLoopback(queueSize, yalogger.NewBaseLogger(nil).NewLogger())
}

// TestLoopbackEnqueueRefusesInsteadOfBlockingWhenFull pins the load-bearing
// queue contract: a full loopback queue answers ErrOutgoingQueueFull at
// once, because the engine treats that refusal as an infrastructure
// redispatch, and a blocking enqueue would wedge the engine dispatch loop.
func TestLoopbackEnqueueRefusesInsteadOfBlockingWhenFull(t *testing.T) {
	t.Parallel()

	lb := newTestLoopback(loopbackQueueOne)

	if err := lb.EnqueueMessage(&protocol.Heartbeat{}); err != nil {
		t.Fatalf("first executor-bound enqueue failed: %v", err)
	}

	if err := lb.enqueueToEngine(&protocol.Heartbeat{}); err != nil {
		t.Fatalf("first engine-bound enqueue failed: %v", err)
	}

	directions := map[string]func(msg protocol.Message) yaerrors.Error{
		"executor-bound": lb.EnqueueMessage,
		"engine-bound":   lb.enqueueToEngine,
	}

	for name, enqueue := range directions {
		result := make(chan yaerrors.Error, 1)

		go func() {
			result <- enqueue(&protocol.Heartbeat{})
		}()

		select {
		case err := <-result:
			if err == nil {
				t.Fatalf("%s enqueue on a full queue reported success", name)
			}

			if !errors.Is(err, ErrOutgoingQueueFull) {
				t.Fatalf("%s enqueue error = %v, want ErrOutgoingQueueFull", name, err)
			}
		case <-time.After(loopbackWaitShort):
			t.Fatalf("%s enqueue blocked on a full queue", name)
		}
	}
}

// TestLoopbackStopRefusesWithoutClosingChannels pins the shutdown contract:
// a stopped loopback refuses messages through the closed latch, and its
// data channels stay open, because a send on a closed channel panics and
// that panic inside a recovered user function would masquerade as a
// function panic.
func TestLoopbackStopRefusesWithoutClosingChannels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		start bool
	}{
		{name: "stopped before start", start: false},
		{name: "stopped after start", start: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			lb := newTestLoopback(loopbackQueueOne)

			if testCase.start {
				lb.start(
					func(_ protocol.Message) {},
					func(_ protocol.Message) {},
				)
			}

			lb.Stop()

			if err := lb.EnqueueMessage(&protocol.Heartbeat{}); !errors.Is(
				err,
				ErrLoopbackStopped,
			) {
				t.Fatalf("executor-bound enqueue error = %v, want ErrLoopbackStopped", err)
			}

			if err := lb.enqueueToEngine(&protocol.Heartbeat{}); !errors.Is(
				err,
				ErrLoopbackStopped,
			) {
				t.Fatalf("engine-bound enqueue error = %v, want ErrLoopbackStopped", err)
			}

			select {
			case lb.toExecutor <- &protocol.Heartbeat{}:
			default:
				t.Fatal("executor queue refused a buffered send after stop")
			}

			select {
			case lb.toEngine <- &protocol.Heartbeat{}:
			default:
				t.Fatal("engine queue refused a buffered send after stop")
			}
		})
	}
}

// TestLocalRuntimeSinkGoesThroughLoopback pins the structural guarantee
// that the runtime reaches the engine only through the loopback queue: a
// sink calling engine handlers directly would re-enter the engine from the
// runtime's goroutines on a code path written for an async transport.
func TestLocalRuntimeSinkGoesThroughLoopback(t *testing.T) {
	t.Parallel()

	local, err := NewLocal(&LocalConfig{ExecutorType: sinkExecutorType}, NewRegistry(), nil)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	msg := &protocol.ExecResult{ExecutionID: 1, AttemptID: 1, Success: true}

	if sinkErr := local.runtime.sink(msg); sinkErr != nil {
		t.Fatalf("sink failed: %v", sinkErr)
	}

	select {
	case got := <-local.loopback.toEngine:
		if got != msg {
			t.Fatalf("sink delivered %T, want the exact enqueued message", got)
		}
	default:
		t.Fatal("sink bypassed the loopback queue")
	}
}

// upsertOnlyEngine stubs the one engine call UpsertJob makes, so a test
// can settle a result synchronously inside the upsert hand-off itself -
// the fastest completion an in-process function can achieve.
type upsertOnlyEngine struct {
	engine.Engine

	onUpsert func(upsert *protocol.JobUpsert) *protocol.JobUpsertAck
}

func (e *upsertOnlyEngine) HandleJobUpsert(
	_ context.Context,
	_ protocol.InstanceID,
	upsert *protocol.JobUpsert,
) *protocol.JobUpsertAck {
	return e.onUpsert(upsert)
}

// TestLocalRegistersWaiterBeforeUpsertReachesEngine pins the registration
// order UpsertJob must keep: the result waiter exists before the upsert is
// handed to the engine. In process a fast function can settle before
// UpsertJob returns, and a waiter registered after the hand-off misses
// that delivery; redelivery would eventually recover it, so only this
// synchronous-delivery stub can prove the order deterministically.
func TestLocalRegistersWaiterBeforeUpsertReachesEngine(t *testing.T) {
	t.Parallel()

	local, err := NewLocal(&LocalConfig{ExecutorType: sinkExecutorType}, NewRegistry(), nil)
	if err != nil {
		t.Fatalf("NewLocal failed: %v", err)
	}

	payload, encodeErr := yaencoding.EncodeMessagePack(upsertStubValue)
	if encodeErr != nil {
		t.Fatalf("payload should encode: %v", encodeErr)
	}

	local.engine = &upsertOnlyEngine{
		onUpsert: func(upsert *protocol.JobUpsert) *protocol.JobUpsertAck {
			local.runtime.handleResultDelivery(&protocol.ResultDelivery{
				JobUUID:     upsert.JobUUID,
				ExecutionID: upsertStubExecutionID,
				Success:     true,
				HasValue:    true,
				Result:      payload,
			})

			return &protocol.JobUpsertAck{
				JobKey:   upsert.JobKey,
				JobUUID:  upsert.JobUUID,
				Accepted: true,
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), loopbackWaitShort)
	defer cancel()

	submission, upsertErr := local.UpsertJob(ctx, &JobSpec{
		Function:   protocol.FunctionSpec{Name: upsertStubFunction},
		Schedule:   protocol.ScheduleSpec{Kind: protocol.ScheduleKindOneShot},
		ResultMode: protocol.ResultModeDeliver,
	})
	if upsertErr != nil {
		t.Fatalf("UpsertJob failed: %v", upsertErr)
	}

	result, awaitErr := submission.Await(ctx)
	if awaitErr != nil {
		t.Fatalf(
			"Await failed: %v: a result settled during the upsert hand-off "+
				"must find its waiter already registered",
			awaitErr,
		)
	}

	value, decodeErr := DecodeResult[int64](result)
	if decodeErr != nil {
		t.Fatalf("DecodeResult failed: %v", decodeErr)
	}

	if *value != upsertStubValue {
		t.Fatalf("value = %d, want %d", *value, upsertStubValue)
	}
}
