package protocol_test

import (
	"bytes"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

func benchmarkExecRequest() *protocol.ExecRequest {
	return &protocol.ExecRequest{
		JobID:             testJobID,
		ExecutionID:       testExecutionID,
		AttemptID:         testAttemptID,
		AttemptNumber:     testAttemptNumber,
		Function:          testFunctionSpec(),
		Args:              bytes.Repeat([]byte{0xAB}, 256),
		ScheduledUnixNano: testScheduledNanos,
		TimeoutMillis:     testTimeoutMillis,
	}
}

func BenchmarkEncodeFrame(b *testing.B) {
	msg := benchmarkExecRequest()

	b.ReportAllocs()

	for b.Loop() {
		if _, err := protocol.EncodeFrame(
			testCorrelationID,
			msg,
			protocol.Limits{},
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeMessage(b *testing.B) {
	payload := benchmarkExecRequest().MarshalPayload()

	b.ReportAllocs()

	for b.Loop() {
		if _, err := protocol.DecodeMessage(
			protocol.MessageTypeExecRequest,
			payload,
			protocol.Limits{},
		); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadMessage(b *testing.B) {
	frame, err := protocol.EncodeFrame(
		testCorrelationID,
		benchmarkExecRequest(),
		protocol.Limits{},
	)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		if _, _, readErr := protocol.ReadMessage(
			bytes.NewReader(frame),
			protocol.Limits{},
		); readErr != nil {
			b.Fatal(readErr)
		}
	}
}
