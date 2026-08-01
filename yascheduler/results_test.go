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
	resultFunctionName protocol.FunctionName = "remote-report"

	resultExecutionID       protocol.ExecutionID = 77
	resultSecondExecutionID protocol.ExecutionID = 78

	resultSum = 5
)

// upsertOverConn submits spec through the running client while acting as
// the scheduler on conn: it consumes the JobUpsert frame, acknowledges it
// accepted, and returns the submission together with the upsert the wire
// carried.
func upsertOverConn(
	t *testing.T,
	running *runningClient,
	conn net.Conn,
	spec *yascheduler.JobSpec,
) (*yascheduler.Submission, *protocol.JobUpsert) {
	t.Helper()

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

		submission, err := running.client.UpsertJob(upsertCtx, spec)
		outcome <- upsertOutcome{submission: submission, err: err}
	}()

	header, upsert := waitForMessage[*protocol.JobUpsert](t, conn)

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

		return result.submission, upsert
	case <-time.After(testReadTimeout):
		t.Fatal("UpsertJob did not finish")

		return nil, nil
	}
}

func deliverSpec(resultMode protocol.ResultMode) *yascheduler.JobSpec {
	return &yascheduler.JobSpec{
		Function: protocol.FunctionSpec{Name: resultFunctionName},
		Schedule: protocol.ScheduleSpec{
			Kind:          protocol.ScheduleKindOneShot,
			StartUnixNano: time.Now().UTC().UnixNano(),
		},
		ResultMode: resultMode,
	}
}

func TestClientResultWaiterSurvivesReconnect(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	running := startClient(t, fs, yascheduler.NewRegistry())

	firstConn, _ := acceptAndRegister(t, fs)

	awaitCtx, awaitCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer awaitCancel()

	if err := running.client.AwaitReady(awaitCtx); err != nil {
		t.Fatalf("AwaitReady failed: %v", err)
	}

	submission, upsert := upsertOverConn(
		t,
		running,
		firstConn,
		deliverSpec(protocol.ResultModeDeliver),
	)

	_ = firstConn.Close()

	secondConn, _ := acceptAndRegister(t, fs)
	defer func() { _ = secondConn.Close() }()

	writeMessage(t, secondConn, 1, &protocol.ResultDelivery{
		JobUUID:     upsert.JobUUID,
		ExecutionID: resultExecutionID,
		Success:     true,
		HasValue:    true,
		Result:      mustEncode(t, addResult{Sum: resultSum}),
	})

	_, ack := waitForMessage[*protocol.ResultDeliveryAck](t, secondConn)

	if !ack.Accepted {
		t.Fatal("delivery to a registered waiter was not accepted")
	}

	if ack.JobUUID != upsert.JobUUID {
		t.Fatalf("ack job uuid = %s, want %s", ack.JobUUID, upsert.JobUUID)
	}

	resultCtx, resultCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer resultCancel()

	result, err := submission.Await(resultCtx)
	if err != nil {
		t.Fatalf("Await failed: %v", err)
	}

	if !result.Success || result.ExecutionID != resultExecutionID {
		t.Fatalf(
			"result success = %t execution = %d, want success execution %d",
			result.Success,
			result.ExecutionID,
			resultExecutionID,
		)
	}

	value, decodeErr := yascheduler.DecodeResult[addResult](result)
	if decodeErr != nil {
		t.Fatalf("DecodeResult failed: %v", decodeErr)
	}

	if value.Sum != resultSum {
		t.Fatalf("sum = %d, want %d", value.Sum, resultSum)
	}
}

func TestClientAcksResultWithoutWaiter(t *testing.T) {
	t.Parallel()

	fs := startFakeScheduler(t)
	startClient(t, fs, yascheduler.NewRegistry())

	conn, _ := acceptAndRegister(t, fs)
	defer func() { _ = conn.Close() }()

	writeMessage(t, conn, 1, &protocol.ResultDelivery{
		JobUUID:     testJobUUID,
		ExecutionID: resultExecutionID,
		Success:     true,
		HasValue:    true,
		Result:      mustEncode(t, addResult{Sum: resultSum}),
	})

	_, ack := waitForMessage[*protocol.ResultDeliveryAck](t, conn)

	if ack.Accepted {
		t.Fatal("delivery without a waiter was accepted: the refusal is what " +
			"stops the scheduler redelivering an abandoned result")
	}

	if ack.JobUUID != testJobUUID {
		t.Fatalf("ack job uuid = %s, want %s", ack.JobUUID, testJobUUID)
	}
}

func TestClientDropsDuplicateResult(t *testing.T) {
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

	submission, upsert := upsertOverConn(
		t,
		running,
		conn,
		deliverSpec(protocol.ResultModeDeliver),
	)

	for _, executionID := range []protocol.ExecutionID{
		resultExecutionID,
		resultSecondExecutionID,
	} {
		writeMessage(t, conn, 1, &protocol.ResultDelivery{
			JobUUID:     upsert.JobUUID,
			ExecutionID: executionID,
			Success:     true,
			HasValue:    true,
			Result:      mustEncode(t, addResult{Sum: resultSum}),
		})

		_, ack := waitForMessage[*protocol.ResultDeliveryAck](t, conn)

		if !ack.Accepted {
			t.Fatalf("delivery %d was not accepted", executionID)
		}
	}

	resultCtx, resultCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer resultCancel()

	result, err := submission.Await(resultCtx)
	if err != nil {
		t.Fatalf("Await failed: %v", err)
	}

	if result.ExecutionID != resultExecutionID {
		t.Fatalf(
			"execution = %d, want the first delivery %d: a duplicate must be "+
				"discarded, not replace the buffered result",
			result.ExecutionID,
			resultExecutionID,
		)
	}
}

func TestSubmissionAwaitIgnoreMode(t *testing.T) {
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

	submission, _ := upsertOverConn(
		t,
		running,
		conn,
		deliverSpec(protocol.ResultModeIgnore),
	)

	resultCtx, resultCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer resultCancel()

	if _, err := submission.Await(resultCtx); !errors.Is(err, yascheduler.ErrResultNotRequested) {
		t.Fatalf("Await error = %v, want ErrResultNotRequested", err)
	}
}

func TestSubmissionCloseIdempotent(t *testing.T) {
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

	submission, _ := upsertOverConn(
		t,
		running,
		conn,
		deliverSpec(protocol.ResultModeDeliver),
	)

	submission.Close()
	submission.Close()

	resultCtx, resultCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer resultCancel()

	if _, err := submission.Await(resultCtx); !errors.Is(err, yascheduler.ErrSubmissionClosed) {
		t.Fatalf("Await error = %v, want ErrSubmissionClosed", err)
	}
}

func TestDecodeResultVoid(t *testing.T) {
	t.Parallel()

	result := &yascheduler.Result{
		JobUUID:     testJobUUID,
		ExecutionID: resultExecutionID,
		Success:     true,
		HasValue:    false,
	}

	if _, err := yascheduler.DecodeResult[yascheduler.Void](result); !errors.Is(
		err,
		yascheduler.ErrResultHasNoValue,
	) {
		t.Fatalf("DecodeResult error = %v, want ErrResultHasNoValue", err)
	}

	if _, err := yascheduler.DecodeResult[yascheduler.Void](nil); !errors.Is(
		err,
		yascheduler.ErrNilResult,
	) {
		t.Fatalf("DecodeResult(nil) error = %v, want ErrNilResult", err)
	}
}
