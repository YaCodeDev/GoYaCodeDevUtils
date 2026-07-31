package yascheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	stubbornFunction protocol.FunctionName = "stubborn"

	stubbornExecutionID protocol.ExecutionID = 21
	stubbornAttemptID   protocol.AttemptID   = 210

	stubbornCorrelation protocol.CorrelationID = 602

	stubbornDrainTimeout = 100 * time.Millisecond
)

// startRun starts one Run cycle and hands back its result, so a test can
// drive several sequential Run calls on the same client.
func startRun(
	ctx context.Context,
	t *testing.T,
	client *yascheduler.Client,
) chan yaerrors.Error {
	t.Helper()

	done := make(chan yaerrors.Error, 1)

	go func() {
		done <- client.Run(ctx)
	}()

	return done
}

func awaitRun(t *testing.T, done chan yaerrors.Error) yaerrors.Error {
	t.Helper()

	select {
	case err := <-done:
		return err
	case <-time.After(testRunStopTimeout):
		t.Fatal("Run did not return in time")

		return nil
	}
}

// TestClientRunReturnsWhenFunctionIgnoresCancel proves Run honours its own
// contract when a registered function ignores its context. Run drains for
// DrainTimeout, cancels the leftovers, and must then return instead of
// waiting out the function and pinning an fx stop hook past its deadline.
func TestClientRunReturnsWhenFunctionIgnoresCancel(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	registry := yascheduler.NewRegistry()

	started := make(chan struct{}, 1)
	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	if err := yascheduler.RegisterFunction(
		registry,
		stubbornFunction,
		testFunctionVersion,
		func(_ context.Context, _ addArgs) (addResult, error) {
			started <- struct{}{}
			<-release

			return addResult{}, nil
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	config := testClientConfig(fs)
	config.DrainTimeout = stubbornDrainTimeout

	client, err := yascheduler.New(config, registry, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := startRun(ctx, t, client)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, stubbornCorrelation, &protocol.ExecRequest{
		ExecutionID: stubbornExecutionID,
		AttemptID:   stubbornAttemptID,
		Function: protocol.FunctionSpec{
			Name:    stubbornFunction,
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{}),
	})

	select {
	case <-started:
	case <-time.After(testReadTimeout):
		t.Fatal("function never started")
	}

	cancel()

	runErr := awaitRun(t, done)
	if runErr == nil {
		t.Fatal("Run reported success while a function was still running")
	}

	if !errors.Is(runErr, yascheduler.ErrDrainTimeout) {
		t.Fatalf("err = %v, want ErrDrainTimeout", runErr)
	}
}
