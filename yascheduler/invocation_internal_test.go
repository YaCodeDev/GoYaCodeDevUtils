package yascheduler

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	invocationFunction protocol.FunctionName    = "invocation-observer"
	invocationVersion  protocol.FunctionVersion = "v1"

	invocationArgValue = int64(11)

	invocationExecutionID   protocol.ExecutionID = 88
	invocationAttemptID     protocol.AttemptID   = 880
	invocationAttemptNumber uint32               = 3
)

var invocationJobUUID = protocol.JobUUID{0x4a, 0x0b, 0x1d}

// TestRuntimeExposesInvocationOnContext drives one execution request
// through the runtime and asserts the invoked function observes the
// request's own identities through InvocationFromContext.
func TestRuntimeExposesInvocationOnContext(t *testing.T) {
	t.Parallel()

	observed := make(chan *Invocation, 1)
	found := make(chan bool, 1)
	registry := NewRegistry()

	if err := RegisterFunction(
		registry,
		invocationFunction,
		invocationVersion,
		func(ctx context.Context, value int64) (int64, error) {
			invocation, carried := InvocationFromContext(ctx)
			observed <- invocation
			found <- carried

			return value, nil
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	client := newInternalClient(t, registry, configuredInterval)
	drainOutgoing(t, client)

	args, encodeErr := yaencoding.EncodeMessagePack(invocationArgValue)
	if encodeErr != nil {
		t.Fatalf("args should encode: %v", encodeErr)
	}

	client.handleExecRequest(context.Background(), &protocol.ExecRequest{
		JobUUID:       invocationJobUUID,
		ExecutionID:   invocationExecutionID,
		AttemptID:     invocationAttemptID,
		AttemptNumber: invocationAttemptNumber,
		Function: protocol.FunctionSpec{
			Name:    invocationFunction,
			Version: invocationVersion,
		},
		Args: args,
	})

	var invocation *Invocation

	select {
	case invocation = <-observed:
	case <-time.After(internalDrainWait):
		t.Fatal("the registered function was never invoked")
	}

	if carried := <-found; !carried {
		t.Fatal("the invoked function should find an Invocation on its context")
	}

	if invocation == nil {
		t.Fatal("a found Invocation should never be nil")
	}

	if invocation.JobUUID != invocationJobUUID {
		t.Errorf(
			"the invocation should carry the request job: got %s, want %s",
			invocation.JobUUID,
			invocationJobUUID,
		)
	}

	if invocation.ExecutionID != invocationExecutionID {
		t.Errorf(
			"the invocation should carry the request execution: got %d, want %d",
			invocation.ExecutionID,
			invocationExecutionID,
		)
	}

	if invocation.AttemptID != invocationAttemptID {
		t.Errorf(
			"the invocation should carry the request attempt: got %d, want %d",
			invocation.AttemptID,
			invocationAttemptID,
		)
	}

	if invocation.AttemptNumber != invocationAttemptNumber {
		t.Errorf(
			"the invocation should carry the request attempt number: got %d, want %d",
			invocation.AttemptNumber,
			invocationAttemptNumber,
		)
	}

	if invocation.Function.Name != invocationFunction ||
		invocation.Function.Version != invocationVersion {
		t.Errorf(
			"the invocation should carry the request function: got %+v",
			invocation.Function,
		)
	}
}

// TestInvocationFromContextWithoutInvocation pins the accessor's answer on
// a context the runtime never touched.
func TestInvocationFromContextWithoutInvocation(t *testing.T) {
	t.Parallel()

	invocation, found := InvocationFromContext(context.Background())

	if found {
		t.Error("a bare context should carry no invocation")
	}

	if invocation != nil {
		t.Errorf("no invocation should be answered: got %+v", invocation)
	}
}
