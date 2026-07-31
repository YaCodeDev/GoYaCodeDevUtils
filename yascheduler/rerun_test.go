package yascheduler_test

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	rerunExecutionID protocol.ExecutionID   = 20
	rerunAttemptID   protocol.AttemptID     = 200
	rerunCorrelation protocol.CorrelationID = 601
)

// TestClientAcceptsWorkAfterRerun proves a client whose Run returned can be
// run again. Run guards only concurrent calls, so a sequential re-run is a
// supported shape: the second Run must serve work instead of registering
// with the scheduler and then refusing every execution as shutting down.
func TestClientAcceptsWorkAfterRerun(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	registry := yascheduler.NewRegistry()

	if err := yascheduler.RegisterFunction(
		registry,
		testFunctionName,
		testFunctionVersion,
		addFunction,
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	client, err := yascheduler.New(testClientConfig(fs), registry, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstDone := startRun(firstCtx, t, client)

	firstConn, _ := acceptAndRegister(t, fs)

	firstCancel()

	if runErr := awaitRun(t, firstDone); runErr != nil {
		t.Fatalf("first Run failed: %v", runErr)
	}

	_ = firstConn.Close()

	secondCtx, secondCancel := context.WithCancel(context.Background())
	secondDone := startRun(secondCtx, t, client)

	t.Cleanup(func() {
		secondCancel()
		awaitRun(t, secondDone)
	})

	secondConn, _ := acceptAndRegister(t, fs)
	defer func() { _ = secondConn.Close() }()

	writeMessage(t, secondConn, rerunCorrelation, &protocol.ExecRequest{
		ExecutionID: rerunExecutionID,
		AttemptID:   rerunAttemptID,
		Function: protocol.FunctionSpec{
			Name:    testFunctionName,
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{A: 2, B: 3}),
	})

	_, accept := waitForMessage[*protocol.ExecAccept](t, secondConn)
	if !accept.Accepted {
		t.Fatalf(
			"a re-run client refused work: error = %+v, want an accepted execution",
			accept.Error,
		)
	}
}
