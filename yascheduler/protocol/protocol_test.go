package protocol_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
	"testing/iotest"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	testCorrelationID  protocol.CorrelationID = 77
	testExecutionID    protocol.ExecutionID   = 22
	testAttemptID      protocol.AttemptID     = 33
	testAttemptNumber  uint32                 = 2
	testScheduledNanos int64                  = 1721481600000000000
	testTimeoutMillis  uint32                 = 30000
	testCapacity       uint32                 = 16
	testInFlight       uint32                 = 5
	testHeartbeatMs    uint32                 = 5000
	testActiveLabels   uint32                 = 3
	testAnnounceLabel  protocol.Label         = "region:eu-west"
	testWithdrawLabel  protocol.Label         = "region:us-east"
	testPinLabel       protocol.Label         = "gpu:a100"
)

// testJobUUID is a fixed identifier so encoded payloads stay byte-stable
// across runs.
var testJobUUID = protocol.JobUUID{
	0x3f, 0x2a, 0x91, 0x0c, 0x77, 0x14, 0x4b, 0x8e,
	0xa5, 0x60, 0xd1, 0x02, 0x9c, 0x33, 0xe7, 0x18,
}

func testFunctionSpec() protocol.FunctionSpec {
	return protocol.FunctionSpec{
		Name:            "send_report",
		Version:         "v1",
		InputSignature:  "protocol_test.reportArgs",
		OutputSignature: "protocol_test.reportResult",
	}
}

func testMessages() []protocol.Message {
	return []protocol.Message{
		&protocol.Register{
			ProtocolVersion: protocol.CurrentVersion,
			ExecutorType:    "report-service",
			InstanceID:      "instance-1",
			Capacity:        testCapacity,
			Functions:       []protocol.FunctionSpec{testFunctionSpec()},
			Labels:          []protocol.Label{testAnnounceLabel, testPinLabel},
		},
		&protocol.RegisterAck{
			Accepted:                true,
			HeartbeatIntervalMillis: testHeartbeatMs,
		},
		&protocol.RegisterAck{
			Accepted: false,
			Error: &protocol.WireError{
				Code:    protocol.ErrorCodeRegistrationRejected,
				Message: "duplicate instance",
			},
		},
		&protocol.Heartbeat{InFlight: testInFlight},
		&protocol.HeartbeatAck{},
		&protocol.ExecRequest{
			JobUUID:           testJobUUID,
			ExecutionID:       testExecutionID,
			AttemptID:         testAttemptID,
			AttemptNumber:     testAttemptNumber,
			Function:          testFunctionSpec(),
			Args:              []byte{1, 2, 3},
			ScheduledUnixNano: testScheduledNanos,
			TimeoutMillis:     testTimeoutMillis,
			DeliverResult:     true,
		},
		&protocol.ExecAccept{
			ExecutionID: testExecutionID,
			AttemptID:   testAttemptID,
			Accepted:    true,
		},
		&protocol.ExecAccept{
			ExecutionID: testExecutionID,
			AttemptID:   testAttemptID,
			Accepted:    false,
			Error: &protocol.WireError{
				Code:      protocol.ErrorCodeCapacityExhausted,
				Retryable: true,
				Message:   "executor full",
			},
		},
		&protocol.ExecResult{
			ExecutionID: testExecutionID,
			AttemptID:   testAttemptID,
			Success:     true,
			HasValue:    true,
			Result:      []byte{9, 8},
		},
		&protocol.ExecResult{
			ExecutionID: testExecutionID,
			AttemptID:   testAttemptID,
			Success:     true,
			HasValue:    false,
		},
		&protocol.ExecResult{
			ExecutionID: testExecutionID,
			AttemptID:   testAttemptID,
			Success:     false,
			Error: &protocol.WireError{
				Code:      protocol.ErrorCodeFunctionError,
				Retryable: true,
				Message:   "boom",
			},
		},
		&protocol.Cancel{
			ExecutionID: testExecutionID,
			AttemptID:   testAttemptID,
			Reason:      "job disabled",
		},
		&protocol.Fault{
			Cause: protocol.WireError{
				Code:    protocol.ErrorCodeUnsupportedVersion,
				Message: "version 9 not supported",
			},
		},
		&protocol.JobUpsert{
			JobUUID:      testJobUUID,
			JobKey:       "report-daily",
			ExecutorType: "report-service",
			Function:     testFunctionSpec(),
			Args:         []byte{4, 5},
			Schedule: protocol.ScheduleSpec{
				Kind:           protocol.ScheduleKindFixedInterval,
				StartUnixNano:  testScheduledNanos,
				IntervalMillis: 60000,
			},
			Enabled: true,
			Backfill: protocol.BackfillSpec{
				Mode:         protocol.BackfillModeEnabled,
				MaxCount:     10,
				MaxAgeMillis: 3600000,
			},
			Retry: protocol.RetrySpec{
				Policy:             protocol.RetryPolicyExponential,
				MaxRetries:         protocol.DefaultMaxRetries,
				InitialDelayMillis: 1000,
				MaxDelayMillis:     60000,
				MultiplierBits:     0x4000000000000000,
			},
			Overlap: protocol.OverlapPolicySkip,
			Pin: protocol.PinSpec{
				Label:  testPinLabel,
				Policy: protocol.PinPolicyPreferred,
			},
			ResultMode: protocol.ResultModeDeliver,
		},
		&protocol.JobUpsertAck{
			JobKey:   "report-daily",
			JobUUID:  testJobUUID,
			Accepted: true,
		},
		&protocol.Shutdown{Reason: "draining"},
		&protocol.LabelUpdate{
			Announce: []protocol.Label{testAnnounceLabel, testPinLabel},
			Withdraw: []protocol.Label{testWithdrawLabel},
		},
		&protocol.LabelUpdate{},
		&protocol.LabelUpdateAck{
			Accepted:    true,
			ActiveCount: testActiveLabels,
		},
		&protocol.LabelUpdateAck{
			Accepted: false,
			Error: &protocol.WireError{
				Code:    protocol.ErrorCodeLabelRejected,
				Message: "label not permitted",
			},
		},
		&protocol.ResultDelivery{
			JobUUID:     testJobUUID,
			ExecutionID: testExecutionID,
			Success:     true,
			HasValue:    true,
			Result:      []byte{7, 6, 5},
		},
		&protocol.ResultDelivery{
			JobUUID:     testJobUUID,
			ExecutionID: testExecutionID,
			Success:     false,
			Error: &protocol.WireError{
				Code:      protocol.ErrorCodeFunctionError,
				Retryable: false,
				Message:   "boom",
			},
		},
		&protocol.ResultDeliveryAck{
			JobUUID:  testJobUUID,
			Accepted: true,
		},
	}
}

func encodeFrameOrFatal(t *testing.T, msg protocol.Message) []byte {
	t.Helper()

	frame, err := protocol.EncodeFrame(testCorrelationID, msg, protocol.Limits{})
	if err != nil {
		t.Fatalf("EncodeFrame failed: %v", err)
	}

	return frame
}

func TestFrameRoundTripAllMessages(t *testing.T) {
	t.Parallel()

	for _, msg := range testMessages() {
		frame := encodeFrameOrFatal(t, msg)

		header, decoded, err := protocol.ReadMessage(
			bytes.NewReader(frame),
			protocol.Limits{},
		)
		if err != nil {
			t.Fatalf("type %d: ReadMessage failed: %v", msg.Type(), err)
		}

		if header.Type != msg.Type() {
			t.Fatalf("header type = %d, want %d", header.Type, msg.Type())
		}

		if header.CorrelationID != testCorrelationID {
			t.Fatalf(
				"correlation id = %d, want %d",
				header.CorrelationID,
				testCorrelationID,
			)
		}

		if !bytes.Equal(decoded.MarshalPayload(), msg.MarshalPayload()) {
			t.Fatalf("type %d: payload did not round-trip", msg.Type())
		}
	}
}

func TestReadFramePartialReads(t *testing.T) {
	t.Parallel()

	msg := &protocol.ExecRequest{
		JobUUID:     testJobUUID,
		ExecutionID: testExecutionID,
		AttemptID:   testAttemptID,
		Function:    testFunctionSpec(),
		Args:        []byte{1, 2, 3, 4, 5},
	}
	frame := encodeFrameOrFatal(t, msg)

	_, decoded, err := protocol.ReadMessage(
		iotest.OneByteReader(bytes.NewReader(frame)),
		protocol.Limits{},
	)
	if err != nil {
		t.Fatalf("ReadMessage over one-byte reader failed: %v", err)
	}

	if !bytes.Equal(decoded.MarshalPayload(), msg.MarshalPayload()) {
		t.Fatal("payload did not survive byte-at-a-time reads")
	}
}

func TestReadFrameMultipleFramesInOneBuffer(t *testing.T) {
	t.Parallel()

	var stream bytes.Buffer

	sent := testMessages()
	for _, msg := range sent {
		stream.Write(encodeFrameOrFatal(t, msg))
	}

	for i, want := range sent {
		_, decoded, err := protocol.ReadMessage(&stream, protocol.Limits{})
		if err != nil {
			t.Fatalf("frame %d: ReadMessage failed: %v", i, err)
		}

		if decoded.Type() != want.Type() {
			t.Fatalf("frame %d: type = %d, want %d", i, decoded.Type(), want.Type())
		}
	}

	if stream.Len() != 0 {
		t.Fatalf("stream has %d leftover bytes", stream.Len())
	}
}

func TestReadFrameRejectsBadMagic(t *testing.T) {
	t.Parallel()

	frame := encodeFrameOrFatal(t, &protocol.Heartbeat{InFlight: 1})
	frame[0] = 'X'

	_, _, err := protocol.ReadFrame(bytes.NewReader(frame), protocol.Limits{})
	if err == nil || !errors.Is(err, protocol.ErrBadMagic) {
		t.Fatalf("err = %v, want ErrBadMagic", err)
	}
}

// TestReadFrameRejectsUnsupportedVersion proves the receiver fails closed
// on any version byte other than the one it speaks. Version1 is covered
// explicitly: protocol 2 does not negotiate and does not downgrade, so a
// v1 frame is as unacceptable as an unknown future one.
func TestReadFrameRejectsUnsupportedVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		version uint8
	}{
		{name: "superseded version 1", version: protocol.Version1},
		{name: "unknown future version", version: protocol.CurrentVersion + 1},
		{name: "zero version", version: 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			frame := encodeFrameOrFatal(t, &protocol.Heartbeat{InFlight: 1})
			frame[4] = testCase.version

			_, _, err := protocol.ReadFrame(bytes.NewReader(frame), protocol.Limits{})
			if err == nil || !errors.Is(err, protocol.ErrUnsupportedVersion) {
				t.Fatalf("err = %v, want ErrUnsupportedVersion", err)
			}
		})
	}
}

func TestCurrentVersionIsVersion2(t *testing.T) {
	t.Parallel()

	if protocol.CurrentVersion != protocol.Version2 {
		t.Fatalf(
			"CurrentVersion = %d, want Version2 (%d)",
			protocol.CurrentVersion,
			protocol.Version2,
		)
	}
}

func TestReadFrameRejectsReservedFlags(t *testing.T) {
	t.Parallel()

	frame := encodeFrameOrFatal(t, &protocol.Heartbeat{InFlight: 1})
	frame[6] = 1

	_, _, err := protocol.ReadFrame(bytes.NewReader(frame), protocol.Limits{})
	if err == nil || !errors.Is(err, protocol.ErrReservedFlags) {
		t.Fatalf("err = %v, want ErrReservedFlags", err)
	}
}

func TestReadFrameRejectsOversizedPayloadLength(t *testing.T) {
	t.Parallel()

	frame := encodeFrameOrFatal(t, &protocol.Heartbeat{InFlight: 1})
	binary.BigEndian.PutUint32(frame[16:], protocol.DefaultMaxFrameSize+1)

	_, _, err := protocol.ReadFrame(bytes.NewReader(frame), protocol.Limits{})
	if err == nil || !errors.Is(err, protocol.ErrFrameTooLarge) {
		t.Fatalf("err = %v, want ErrFrameTooLarge", err)
	}
}

func TestReadFrameTruncatedPayload(t *testing.T) {
	t.Parallel()

	frame := encodeFrameOrFatal(t, &protocol.Cancel{
		ExecutionID: testExecutionID,
		AttemptID:   testAttemptID,
		Reason:      "truncated",
	})

	_, _, err := protocol.ReadFrame(
		bytes.NewReader(frame[:len(frame)-3]),
		protocol.Limits{},
	)
	if err == nil {
		t.Fatal("truncated payload decoded successfully")
	}
}

func TestReadFrameTruncatedHeader(t *testing.T) {
	t.Parallel()

	frame := encodeFrameOrFatal(t, &protocol.HeartbeatAck{})

	_, _, err := protocol.ReadFrame(
		bytes.NewReader(frame[:protocol.HeaderSize-1]),
		protocol.Limits{},
	)
	if err == nil {
		t.Fatal("truncated header decoded successfully")
	}
}

func TestDecodeMessageUnknownType(t *testing.T) {
	t.Parallel()

	_, err := protocol.DecodeMessage(protocol.MessageType(200), nil, protocol.Limits{})
	if err == nil || !errors.Is(err, protocol.ErrUnknownMessageType) {
		t.Fatalf("err = %v, want ErrUnknownMessageType", err)
	}
}

func TestDecodeMessageTrailingBytes(t *testing.T) {
	t.Parallel()

	payload := (&protocol.Heartbeat{InFlight: 1}).MarshalPayload()
	payload = append(payload, 0xFF)

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeHeartbeat,
		payload,
		protocol.Limits{},
	)
	if err == nil || !errors.Is(err, protocol.ErrTrailingBytes) {
		t.Fatalf("err = %v, want ErrTrailingBytes", err)
	}
}

func TestDecodeMessageInvalidBoolByte(t *testing.T) {
	t.Parallel()

	payload := (&protocol.RegisterAck{Accepted: true}).MarshalPayload()
	payload[0] = 7

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeRegisterAck,
		payload,
		protocol.Limits{},
	)
	if err == nil || !errors.Is(err, protocol.ErrInvalidBool) {
		t.Fatalf("err = %v, want ErrInvalidBool", err)
	}
}

func TestDecodeMessageStringLimit(t *testing.T) {
	t.Parallel()

	limits := protocol.Limits{MaxStringLen: 4}
	payload := (&protocol.Shutdown{Reason: "definitely-longer-than-four"}).MarshalPayload()

	_, err := protocol.DecodeMessage(protocol.MessageTypeShutdown, payload, limits)
	if err == nil || !errors.Is(err, protocol.ErrStringTooLong) {
		t.Fatalf("err = %v, want ErrStringTooLong", err)
	}
}

func TestDecodeMessageBytesLimit(t *testing.T) {
	t.Parallel()

	limits := protocol.Limits{MaxBytesLen: 2}
	msg := &protocol.ExecResult{
		ExecutionID: testExecutionID,
		AttemptID:   testAttemptID,
		Success:     true,
		Result:      []byte{1, 2, 3, 4},
	}

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeExecResult,
		msg.MarshalPayload(),
		limits,
	)
	if err == nil || !errors.Is(err, protocol.ErrBytesTooLong) {
		t.Fatalf("err = %v, want ErrBytesTooLong", err)
	}
}

func TestDecodeMessageFunctionCountLimit(t *testing.T) {
	t.Parallel()

	limits := protocol.Limits{MaxFunctions: 1}
	msg := &protocol.Register{
		ProtocolVersion: protocol.CurrentVersion,
		ExecutorType:    "report-service",
		InstanceID:      "instance-1",
		Functions: []protocol.FunctionSpec{
			testFunctionSpec(),
			testFunctionSpec(),
		},
	}

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeRegister,
		msg.MarshalPayload(),
		limits,
	)
	if err == nil || !errors.Is(err, protocol.ErrTooManyFunctions) {
		t.Fatalf("err = %v, want ErrTooManyFunctions", err)
	}
}

func TestDecodeMessageLabelLimit(t *testing.T) {
	t.Parallel()

	limits := protocol.Limits{MaxLabels: 1}
	msg := &protocol.LabelUpdate{
		Announce: []protocol.Label{testAnnounceLabel, testPinLabel},
	}

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeLabelUpdate,
		msg.MarshalPayload(),
		limits,
	)
	if err == nil || !errors.Is(err, protocol.ErrTooManyLabels) {
		t.Fatalf("err = %v, want ErrTooManyLabels", err)
	}
}

func TestDecodeMessageLabelLengthLimit(t *testing.T) {
	t.Parallel()

	limits := protocol.Limits{MaxLabelLen: 4}
	msg := &protocol.LabelUpdate{
		Announce: []protocol.Label{testAnnounceLabel},
	}

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeLabelUpdate,
		msg.MarshalPayload(),
		limits,
	)
	if err == nil || !errors.Is(err, protocol.ErrLabelTooLong) {
		t.Fatalf("err = %v, want ErrLabelTooLong", err)
	}
}

// TestDecodeMessageResultBytesLimit proves a delivered result is bounded by
// MaxResultBytes rather than by MaxBytesLen. The two limits are separate
// because a held result outlives its message.
func TestDecodeMessageResultBytesLimit(t *testing.T) {
	t.Parallel()

	limits := protocol.Limits{MaxResultBytes: 2}
	msg := &protocol.ResultDelivery{
		JobUUID:     testJobUUID,
		ExecutionID: testExecutionID,
		Success:     true,
		HasValue:    true,
		Result:      []byte{1, 2, 3, 4},
	}

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeResultDelivery,
		msg.MarshalPayload(),
		limits,
	)
	if err == nil || !errors.Is(err, protocol.ErrResultTooLarge) {
		t.Fatalf("err = %v, want ErrResultTooLarge", err)
	}
}

func TestDecodeMessageShortJobUUID(t *testing.T) {
	t.Parallel()

	payload := (&protocol.ResultDeliveryAck{JobUUID: testJobUUID}).MarshalPayload()

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeResultDeliveryAck,
		payload[:len(payload)-2],
		protocol.Limits{},
	)
	if err == nil || !errors.Is(err, protocol.ErrShortUUID) {
		t.Fatalf("err = %v, want ErrShortUUID", err)
	}
}

func TestJobUUIDStringAndIsZero(t *testing.T) {
	t.Parallel()

	const wantString = "3f2a910c-7714-4b8e-a560-d1029c33e718"

	if got := testJobUUID.String(); got != wantString {
		t.Fatalf("String() = %q, want %q", got, wantString)
	}

	if testJobUUID.IsZero() {
		t.Fatal("a minted identifier reported itself zero")
	}

	if !(protocol.JobUUID{}).IsZero() {
		t.Fatal("the zero identifier did not report itself zero")
	}
}

func TestDecodeMessageDeclaredLengthPastEnd(t *testing.T) {
	t.Parallel()

	payload := (&protocol.Shutdown{Reason: "ok"}).MarshalPayload()
	binary.BigEndian.PutUint32(payload[0:], 1000)

	_, err := protocol.DecodeMessage(
		protocol.MessageTypeShutdown,
		payload,
		protocol.Limits{},
	)
	if err == nil || !errors.Is(err, protocol.ErrShortBuffer) {
		t.Fatalf("err = %v, want ErrShortBuffer", err)
	}
}

func TestEncodeFrameRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	limits := protocol.Limits{MaxFrameSize: 8}
	msg := &protocol.Shutdown{Reason: "way-too-long-for-eight-bytes"}

	_, err := protocol.EncodeFrame(testCorrelationID, msg, limits)
	if err == nil || !errors.Is(err, protocol.ErrFrameTooLarge) {
		t.Fatalf("err = %v, want ErrFrameTooLarge", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

func TestWriteFrameDetectsShortWrite(t *testing.T) {
	t.Parallel()

	err := protocol.WriteFrame(
		shortWriter{},
		testCorrelationID,
		&protocol.HeartbeatAck{},
		protocol.Limits{},
	)
	if err == nil || !errors.Is(err, protocol.ErrShortWrite) {
		t.Fatalf("err = %v, want ErrShortWrite", err)
	}
}

func TestWriteFrameThenReadMessage(t *testing.T) {
	t.Parallel()

	var stream bytes.Buffer

	msg := &protocol.Heartbeat{InFlight: testInFlight}

	if err := protocol.WriteFrame(
		&stream,
		testCorrelationID,
		msg,
		protocol.Limits{},
	); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	_, decoded, err := protocol.ReadMessage(&stream, protocol.Limits{})
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	heartbeat, ok := decoded.(*protocol.Heartbeat)
	if !ok {
		t.Fatalf("decoded type = %T, want *protocol.Heartbeat", decoded)
	}

	if heartbeat.InFlight != testInFlight {
		t.Fatalf("in flight = %d, want %d", heartbeat.InFlight, testInFlight)
	}
}
