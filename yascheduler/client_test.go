package yascheduler_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	testExecutorType   protocol.ExecutorType = "test-executor"
	testHeartbeatMilli uint32                = 20
	testReadTimeout                          = 2 * time.Second
	testRunStopTimeout                       = 5 * time.Second
)

// testJobUUID stands in for a scheduler-side job identity in requests the
// fake scheduler sends.
var testJobUUID = protocol.JobUUID{
	0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x47, 0x88,
	0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x01,
}

type fakeScheduler struct {
	listener net.Listener
	conns    chan net.Conn
}

func startFakeScheduler(t *testing.T) *fakeScheduler {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	fs := &fakeScheduler{
		listener: listener,
		conns:    make(chan net.Conn, 8),
	}

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				close(fs.conns)

				return
			}

			fs.conns <- conn
		}
	}()

	t.Cleanup(func() { _ = listener.Close() })

	return fs
}

func (fs *fakeScheduler) addr() string {
	return fs.listener.Addr().String()
}

func (fs *fakeScheduler) nextConn(t *testing.T) net.Conn {
	t.Helper()

	select {
	case conn, open := <-fs.conns:
		if !open {
			t.Fatal("listener closed before connection arrived")
		}

		return conn
	case <-time.After(testReadTimeout):
		t.Fatal("no connection arrived in time")

		return nil
	}
}

func readMessage(t *testing.T, conn net.Conn) (protocol.Header, protocol.Message) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(testReadTimeout)); err != nil {
		t.Fatalf("set read deadline failed: %v", err)
	}

	header, msg, err := protocol.ReadMessage(conn, protocol.Limits{})
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}

	return header, msg
}

func writeMessage(
	t *testing.T,
	conn net.Conn,
	correlationID protocol.CorrelationID,
	msg protocol.Message,
) {
	t.Helper()

	if err := protocol.WriteFrame(conn, correlationID, msg, protocol.Limits{}); err != nil {
		t.Fatalf("write frame failed: %v", err)
	}
}

// waitForMessage reads frames until one matches T, answering heartbeats
// on the way so the client's read deadline logic stays satisfied.
func waitForMessage[T protocol.Message](
	t *testing.T,
	conn net.Conn,
) (protocol.Header, T) {
	t.Helper()

	deadline := time.Now().Add(testReadTimeout)

	for time.Now().Before(deadline) {
		header, msg := readMessage(t, conn)

		if heartbeat, isHeartbeat := msg.(*protocol.Heartbeat); isHeartbeat {
			_ = heartbeat

			writeMessage(t, conn, header.CorrelationID, &protocol.HeartbeatAck{})

			if _, want := any(msg).(T); !want {
				continue
			}
		}

		if typed, matches := any(msg).(T); matches {
			return header, typed
		}
	}

	t.Fatal("expected message type never arrived")

	var zero T

	return protocol.Header{}, zero
}

func acceptAndRegister(
	t *testing.T,
	fs *fakeScheduler,
) (net.Conn, *protocol.Register) {
	t.Helper()

	conn := fs.nextConn(t)

	header, msg := readMessage(t, conn)

	register, isRegister := msg.(*protocol.Register)
	if !isRegister {
		t.Fatalf("first message type = %T, want *protocol.Register", msg)
	}

	writeMessage(t, conn, header.CorrelationID, &protocol.RegisterAck{
		Accepted:                true,
		HeartbeatIntervalMillis: testHeartbeatMilli,
	})

	return conn, register
}

func testClientConfig(fs *fakeScheduler) *yascheduler.Config {
	return &yascheduler.Config{
		Address:                  fs.addr(),
		ExecutorType:             testExecutorType,
		Capacity:                 2,
		HeartbeatInterval:        20 * time.Millisecond,
		ReconnectInitialInterval: 10 * time.Millisecond,
		ReconnectMaxInterval:     50 * time.Millisecond,
		DrainTimeout:             time.Second,
	}
}

type runningClient struct {
	client *yascheduler.Client
	cancel context.CancelFunc
	done   chan struct{}
}

func startClient(
	t *testing.T,
	fs *fakeScheduler,
	registry *yascheduler.Registry,
) *runningClient {
	t.Helper()

	client, err := yascheduler.New(testClientConfig(fs), registry, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		if runErr := client.Run(ctx); runErr != nil {
			t.Errorf("Run returned error: %v", runErr)
		}
	}()

	running := &runningClient{client: client, cancel: cancel, done: done}

	t.Cleanup(func() { running.stop(t) })

	return running
}

func (rc *runningClient) stop(t *testing.T) {
	t.Helper()

	rc.cancel()

	select {
	case <-rc.done:
	case <-time.After(testRunStopTimeout):
		t.Fatal("client Run did not stop in time")
	}
}

func TestClientRegistersWithFunctions(t *testing.T) {
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

	running := startClient(t, fs, registry)

	conn, register := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	if register.ExecutorType != testExecutorType {
		t.Fatalf(
			"executor type = %q, want %q",
			register.ExecutorType,
			testExecutorType,
		)
	}

	if register.InstanceID != running.client.InstanceID() {
		t.Fatal("instance id does not match client")
	}

	if len(register.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(register.Functions))
	}

	if register.Functions[0].Name != testFunctionName {
		t.Fatalf("function name = %q", register.Functions[0].Name)
	}

	if register.Functions[0].InputSignature == "" {
		t.Fatal("input signature is empty")
	}

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}
}

func TestClientExecutesFunctionAndReportsResult(t *testing.T) {
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

	startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	request := &protocol.ExecRequest{
		JobUUID:       testJobUUID,
		ExecutionID:   10,
		AttemptID:     100,
		AttemptNumber: 1,
		Function: protocol.FunctionSpec{
			Name:    testFunctionName,
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{A: 2, B: 3}),
	}
	writeMessage(t, conn, 500, request)

	_, accept := waitForMessage[*protocol.ExecAccept](t, conn)
	if !accept.Accepted {
		t.Fatalf("execution rejected: %s", accept.Error.Message)
	}

	if accept.ExecutionID != request.ExecutionID ||
		accept.AttemptID != request.AttemptID {
		t.Fatal("accept ids do not match request")
	}

	_, result := waitForMessage[*protocol.ExecResult](t, conn)
	if !result.Success {
		t.Fatalf("execution failed: %s", result.Error.Message)
	}

	if result.ExecutionID != request.ExecutionID ||
		result.AttemptID != request.AttemptID {
		t.Fatal("result ids do not match request")
	}

	sum := mustDecode[addResult](t, result.Result)
	if sum.Sum != 5 {
		t.Fatalf("sum = %d, want 5", sum.Sum)
	}
}

func TestClientRejectsUnknownFunction(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)

	startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 501, &protocol.ExecRequest{
		ExecutionID: 11,
		AttemptID:   110,
		Function: protocol.FunctionSpec{
			Name:    "missing",
			Version: testFunctionVersion,
		},
	})

	_, accept := waitForMessage[*protocol.ExecAccept](t, conn)
	if accept.Accepted {
		t.Fatal("unknown function was accepted")
	}

	if accept.Error == nil ||
		accept.Error.Code != protocol.ErrorCodeUnknownFunction {
		t.Fatalf("error = %+v, want ErrorCodeUnknownFunction", accept.Error)
	}
}

func TestClientRejectsIncompatibleSignature(t *testing.T) {
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

	startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 502, &protocol.ExecRequest{
		ExecutionID: 12,
		AttemptID:   120,
		Function: protocol.FunctionSpec{
			Name:           testFunctionName,
			Version:        testFunctionVersion,
			InputSignature: "some.other.Type",
		},
	})

	_, accept := waitForMessage[*protocol.ExecAccept](t, conn)
	if accept.Accepted {
		t.Fatal("incompatible signature was accepted")
	}

	if accept.Error == nil ||
		accept.Error.Code != protocol.ErrorCodeIncompatibleFunction {
		t.Fatalf("error = %+v, want ErrorCodeIncompatibleFunction", accept.Error)
	}
}

func TestClientReportsFunctionError(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	registry := yascheduler.NewRegistry()

	failure := errors.New("business failure")

	if err := yascheduler.RegisterFunction(
		registry,
		"failing",
		testFunctionVersion,
		func(_ context.Context, _ addArgs) (addResult, error) {
			return addResult{}, failure
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 503, &protocol.ExecRequest{
		ExecutionID: 13,
		AttemptID:   130,
		Function: protocol.FunctionSpec{
			Name:    "failing",
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{}),
	})

	_, accept := waitForMessage[*protocol.ExecAccept](t, conn)
	if !accept.Accepted {
		t.Fatal("execution rejected")
	}

	_, result := waitForMessage[*protocol.ExecResult](t, conn)
	if result.Success {
		t.Fatal("failing function reported success")
	}

	if result.Error == nil || result.Error.Code != protocol.ErrorCodeFunctionError {
		t.Fatalf("error = %+v, want ErrorCodeFunctionError", result.Error)
	}

	if !result.Error.Retryable {
		t.Fatal("plain function error must be retryable")
	}
}

func TestClientReportsNonRetryableFunctionError(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	registry := yascheduler.NewRegistry()

	if err := yascheduler.RegisterFunction(
		registry,
		"fatal",
		testFunctionVersion,
		func(_ context.Context, _ addArgs) (addResult, error) {
			return addResult{}, yascheduler.NonRetryable(errors.New("no retry"))
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 504, &protocol.ExecRequest{
		ExecutionID: 14,
		AttemptID:   140,
		Function: protocol.FunctionSpec{
			Name:    "fatal",
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{}),
	})

	_, result := waitForMessage[*protocol.ExecResult](t, conn)
	if result.Success {
		t.Fatal("fatal function reported success")
	}

	if result.Error == nil || result.Error.Retryable {
		t.Fatalf("error = %+v, want non-retryable", result.Error)
	}
}

func TestClientRecoversFunctionPanic(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	registry := yascheduler.NewRegistry()

	if err := yascheduler.RegisterFunction(
		registry,
		"panicking",
		testFunctionVersion,
		func(_ context.Context, _ addArgs) (addResult, error) {
			panic("kaboom")
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 505, &protocol.ExecRequest{
		ExecutionID: 15,
		AttemptID:   150,
		Function: protocol.FunctionSpec{
			Name:    "panicking",
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{}),
	})

	_, result := waitForMessage[*protocol.ExecResult](t, conn)
	if result.Success {
		t.Fatal("panicking function reported success")
	}

	if result.Error == nil || result.Error.Code != protocol.ErrorCodeFunctionPanic {
		t.Fatalf("error = %+v, want ErrorCodeFunctionPanic", result.Error)
	}
}

func TestClientReportsInvalidArguments(t *testing.T) {
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

	startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 506, &protocol.ExecRequest{
		ExecutionID: 16,
		AttemptID:   160,
		Function: protocol.FunctionSpec{
			Name:    testFunctionName,
			Version: testFunctionVersion,
		},
		Args: []byte{0xC1, 0xFF, 0x00},
	})

	_, result := waitForMessage[*protocol.ExecResult](t, conn)
	if result.Success {
		t.Fatal("invalid arguments reported success")
	}

	if result.Error == nil ||
		result.Error.Code != protocol.ErrorCodeInvalidArguments {
		t.Fatalf("error = %+v, want ErrorCodeInvalidArguments", result.Error)
	}

	if result.Error.Retryable {
		t.Fatal("invalid arguments must not be retryable")
	}
}

func TestClientReconnectsWithSameInstanceID(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)

	running := startClient(t, fs, yascheduler.NewRegistry())

	firstConn, firstRegister := acceptAndRegister(t, fs)

	_ = firstConn.Close()

	secondConn, secondRegister := acceptAndRegister(t, fs)
	defer func() { _ = secondConn.Close() }()

	if firstRegister.InstanceID != secondRegister.InstanceID {
		t.Fatal("instance id changed across reconnect")
	}

	if secondRegister.InstanceID != running.client.InstanceID() {
		t.Fatal("instance id does not match client")
	}
}

func TestClientRetriesAfterRegistrationRejection(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)

	startClient(t, fs, yascheduler.NewRegistry())

	conn := fs.nextConn(t)

	header, msg := readMessage(t, conn)
	if _, isRegister := msg.(*protocol.Register); !isRegister {
		t.Fatalf("first message type = %T", msg)
	}

	writeMessage(t, conn, header.CorrelationID, &protocol.RegisterAck{
		Accepted: false,
		Error: &protocol.WireError{
			Code:    protocol.ErrorCodeRegistrationRejected,
			Message: "not today",
		},
	})

	_ = conn.Close()

	secondConn, _ := acceptAndRegister(t, fs)

	_ = secondConn.Close()
}

func TestClientHeartbeats(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)

	startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	_, heartbeat := waitForMessage[*protocol.Heartbeat](t, conn)
	if heartbeat.InFlight != 0 {
		t.Fatalf("in flight = %d, want 0", heartbeat.InFlight)
	}
}

func TestClientUpsertJob(t *testing.T) {
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

	running := startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	type upsertOutcome struct {
		submission *yascheduler.Submission
		err        error
	}

	outcome := make(chan upsertOutcome, 1)

	go func() {
		upsertCtx, upsertCancel := context.WithTimeout(
			context.Background(),
			testReadTimeout,
		)
		defer upsertCancel()

		submission, upsertErr := running.client.UpsertJob(upsertCtx, &yascheduler.JobSpec{
			Key: "job-a",
			Function: protocol.FunctionSpec{
				Name:    testFunctionName,
				Version: testFunctionVersion,
			},
			Args: addArgs{A: 1, B: 2},
			Schedule: protocol.ScheduleSpec{
				Kind:          protocol.ScheduleKindOneShot,
				StartUnixNano: time.Now().Add(time.Hour).UnixNano(),
			},
		})

		outcome <- upsertOutcome{submission: submission, err: upsertErr}
	}()

	header, upsert := waitForMessage[*protocol.JobUpsert](t, conn)

	if upsert.JobKey != "job-a" {
		t.Fatalf("job key = %q", upsert.JobKey)
	}

	if upsert.ExecutorType != testExecutorType {
		t.Fatalf("executor type = %q", upsert.ExecutorType)
	}

	if upsert.Function.InputSignature == "" {
		t.Fatal("input signature was not stamped from local registry")
	}

	if upsert.JobUUID.IsZero() {
		t.Fatal("upsert carried a zero job uuid: the client must mint one")
	}

	writeMessage(t, conn, header.CorrelationID, &protocol.JobUpsertAck{
		JobKey:   upsert.JobKey,
		JobUUID:  upsert.JobUUID,
		Accepted: true,
	})

	select {
	case result := <-outcome:
		if result.err != nil {
			t.Fatalf("UpsertJob failed: %v", result.err)
		}

		defer result.submission.Close()

		if result.submission.JobUUID != upsert.JobUUID {
			t.Fatalf(
				"job uuid = %s, want %s",
				result.submission.JobUUID,
				upsert.JobUUID,
			)
		}
	case <-time.After(testReadTimeout):
		t.Fatal("UpsertJob did not finish")
	}
}

func TestClientDeleteJob(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	running := startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	type deleteOutcome struct {
		deleted bool
		err     error
	}

	outcome := make(chan deleteOutcome, 1)

	go func() {
		deleteCtx, deleteCancel := context.WithTimeout(
			context.Background(),
			testReadTimeout,
		)
		defer deleteCancel()

		deleted, deleteErr := running.client.DeleteJob(deleteCtx, "", "job-a")

		outcome <- deleteOutcome{deleted: deleted, err: deleteErr}
	}()

	header, del := waitForMessage[*protocol.JobDelete](t, conn)

	if del.JobKey != "job-a" {
		t.Fatalf("job key = %q", del.JobKey)
	}

	if del.ExecutorType != testExecutorType {
		t.Fatalf(
			"an empty executor type should default to the client's own: got %q",
			del.ExecutorType,
		)
	}

	writeMessage(t, conn, header.CorrelationID, &protocol.JobDeleteAck{
		JobKey:  del.JobKey,
		Deleted: true,
	})

	select {
	case result := <-outcome:
		if result.err != nil {
			t.Fatalf("DeleteJob failed: %v", result.err)
		}

		if !result.deleted {
			t.Fatal("the acknowledged delete should report true")
		}
	case <-time.After(testReadTimeout):
		t.Fatal("DeleteJob did not finish")
	}
}

func TestClientDeleteJobRefused(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	running := startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	outcome := make(chan error, 1)

	go func() {
		deleteCtx, deleteCancel := context.WithTimeout(
			context.Background(),
			testReadTimeout,
		)
		defer deleteCancel()

		_, deleteErr := running.client.DeleteJob(deleteCtx, "", "job-refused")

		outcome <- deleteErr
	}()

	header, del := waitForMessage[*protocol.JobDelete](t, conn)

	writeMessage(t, conn, header.CorrelationID, &protocol.JobDeleteAck{
		JobKey: del.JobKey,
		Error: &protocol.WireError{
			Code:    protocol.ErrorCodeMalformedFrame,
			Message: "refused for the test",
		},
	})

	select {
	case deleteErr := <-outcome:
		if deleteErr == nil {
			t.Fatal("a refused delete should surface an error")
		}

		if !errors.Is(deleteErr, yascheduler.ErrDeleteRejected) {
			t.Fatalf("err = %v, want ErrDeleteRejected", deleteErr)
		}
	case <-time.After(testReadTimeout):
		t.Fatal("DeleteJob did not finish")
	}
}

func TestClientCancelsRunningExecution(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	registry := yascheduler.NewRegistry()

	blocked := make(chan struct{})

	if err := yascheduler.RegisterFunction(
		registry,
		"blocking",
		testFunctionVersion,
		func(ctx context.Context, _ addArgs) (addResult, error) {
			close(blocked)
			<-ctx.Done()

			return addResult{}, ctx.Err()
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	startClient(t, fs, registry)

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 507, &protocol.ExecRequest{
		ExecutionID: 17,
		AttemptID:   170,
		Function: protocol.FunctionSpec{
			Name:    "blocking",
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{}),
	})

	_, accept := waitForMessage[*protocol.ExecAccept](t, conn)
	if !accept.Accepted {
		t.Fatal("execution rejected")
	}

	select {
	case <-blocked:
	case <-time.After(testReadTimeout):
		t.Fatal("function never started")
	}

	writeMessage(t, conn, 508, &protocol.Cancel{
		ExecutionID: 17,
		AttemptID:   170,
		Reason:      "test cancel",
	})

	_, result := waitForMessage[*protocol.ExecResult](t, conn)
	if result.Success {
		t.Fatal("cancelled execution reported success")
	}
}

func TestClientGracefulShutdownAnnouncesAndStops(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)

	running := startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	running.cancel()

	_, shutdown := waitForMessage[*protocol.Shutdown](t, conn)
	if shutdown.Reason == "" {
		t.Fatal("shutdown reason is empty")
	}

	select {
	case <-running.done:
	case <-time.After(testRunStopTimeout):
		t.Fatal("Run did not return after cancel")
	}
}

func TestClientCapacityExhaustion(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	registry := yascheduler.NewRegistry()

	release := make(chan struct{})

	if err := yascheduler.RegisterFunction(
		registry,
		"slow",
		testFunctionVersion,
		func(_ context.Context, _ addArgs) (addResult, error) {
			<-release

			return addResult{}, nil
		},
	); err != nil {
		t.Fatalf("RegisterFunction failed: %v", err)
	}

	client, err := yascheduler.New(&yascheduler.Config{
		Address:                  fs.addr(),
		ExecutorType:             testExecutorType,
		Capacity:                 1,
		HeartbeatInterval:        20 * time.Millisecond,
		ReconnectInitialInterval: 10 * time.Millisecond,
		ReconnectMaxInterval:     50 * time.Millisecond,
		DrainTimeout:             time.Second,
	}, registry, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = client.Run(ctx)
	}()

	t.Cleanup(func() {
		close(release)
		cancel()
		<-done
	})

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 509, &protocol.ExecRequest{
		ExecutionID: 18,
		AttemptID:   180,
		Function: protocol.FunctionSpec{
			Name:    "slow",
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{}),
	})

	_, firstAccept := waitForMessage[*protocol.ExecAccept](t, conn)
	if !firstAccept.Accepted {
		t.Fatal("first execution rejected")
	}

	writeMessage(t, conn, 510, &protocol.ExecRequest{
		ExecutionID: 19,
		AttemptID:   190,
		Function: protocol.FunctionSpec{
			Name:    "slow",
			Version: testFunctionVersion,
		},
		Args: mustEncode(t, addArgs{}),
	})

	_, secondAccept := waitForMessage[*protocol.ExecAccept](t, conn)
	if secondAccept.Accepted {
		t.Fatal("second execution accepted above capacity")
	}

	if secondAccept.Error == nil ||
		secondAccept.Error.Code != protocol.ErrorCodeCapacityExhausted {
		t.Fatalf("error = %+v, want ErrorCodeCapacityExhausted", secondAccept.Error)
	}
}
