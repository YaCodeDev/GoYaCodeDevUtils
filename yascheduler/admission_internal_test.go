package yascheduler

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	instantFunction protocol.FunctionName = "admission-instant"
	blockerFunction protocol.FunctionName = "admission-blocker"

	admissionRounds    = 60
	admissionArgValue  = int64(7)
	admissionParkWait  = time.Millisecond
	admissionStepGrain = 5 * time.Microsecond
	admissionSteps     = 12

	admissionExecutionID protocol.ExecutionID = 77
	admissionAttemptID   protocol.AttemptID   = 770
)

// TestExecAdmissionNeverRacesDrain drives one execution request into the
// window between its stopping check and its drain-counter increment, while
// a connection drain latches shutdown and waits on the running execution.
// Admission that is not synchronised with that latch raises the counter
// from zero while the drain waits on it, which sync.WaitGroup defines as
// misuse and answers with a process panic.
func TestExecAdmissionNeverRacesDrain(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	gate := make(chan struct{})
	registry := admissionRegistry(t, started, gate)
	client := newInternalClient(t, registry, configuredInterval)

	args, encodeErr := yaencoding.EncodeMessagePack(admissionArgValue)
	if encodeErr != nil {
		t.Fatalf("args should encode: %v", encodeErr)
	}

	drainOutgoing(t, client)

	ctx := context.Background()

	for round := range admissionRounds {
		client.stopping.Store(false)

		go client.handleExecRequest(ctx, admissionRequest(blockerFunction, args))

		select {
		case <-started:
		case <-time.After(internalDrainWait):
			t.Fatal("blocking execution never started")
		}

		registry.mu.Lock()

		stalled := make(chan struct{})

		go func() {
			defer close(stalled)

			client.handleExecRequest(ctx, admissionRequest(instantFunction, args))
		}()

		time.Sleep(admissionParkWait)

		drained := make(chan struct{})

		go func() {
			defer close(drained)

			client.shutdownConnection()
		}()

		time.Sleep(admissionParkWait)

		gate <- struct{}{}

		time.Sleep(time.Duration(round%admissionSteps) * admissionStepGrain)

		registry.mu.Unlock()

		<-stalled
		<-drained
	}
}

// admissionRegistry holds one execution that blocks on gate and one that
// returns at once, so a test can hold the drain counter above zero while a
// second request is stalled inside admission.
func admissionRegistry(
	t *testing.T,
	started chan struct{},
	gate chan struct{},
) *Registry {
	t.Helper()

	registry := NewRegistry()

	if err := RegisterFunction(
		registry,
		blockerFunction,
		"",
		func(_ context.Context, value int64) (int64, error) {
			started <- struct{}{}
			<-gate

			return value, nil
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	if err := RegisterFunction(
		registry,
		instantFunction,
		"",
		func(_ context.Context, value int64) (int64, error) {
			return value, nil
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	return registry
}

func admissionRequest(name protocol.FunctionName, args []byte) *protocol.ExecRequest {
	return &protocol.ExecRequest{
		ExecutionID: admissionExecutionID,
		AttemptID:   admissionAttemptID,
		Function:    protocol.FunctionSpec{Name: name},
		Args:        args,
	}
}

// drainOutgoing attaches a connection queue and empties it for the whole
// test, so admission never short-circuits on a full outgoing queue.
func drainOutgoing(t *testing.T, client *Client) {
	t.Helper()

	outgoing := make(chan []byte, internalQueueSize)
	client.attachConnection(outgoing)

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })

	go func() {
		for {
			select {
			case <-stop:
				return
			case <-outgoing:
			}
		}
	}()
}
