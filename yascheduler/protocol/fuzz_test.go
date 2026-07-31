package protocol_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

func fuzzSeedFrames() [][]byte {
	seeds := make([][]byte, 0, len(testMessages()))

	for _, msg := range testMessages() {
		frame, err := protocol.EncodeFrame(testCorrelationID, msg, protocol.Limits{})
		if err != nil {
			continue
		}

		seeds = append(seeds, frame)
	}

	return seeds
}

func FuzzReadFrame(f *testing.F) {
	for _, seed := range fuzzSeedFrames() {
		f.Add(seed)
	}

	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{0xFF}, protocol.HeaderSize))

	f.Fuzz(func(t *testing.T, data []byte) {
		header, payload, err := protocol.ReadFrame(
			bytes.NewReader(data),
			protocol.Limits{},
		)
		if err != nil {
			return
		}

		if int(header.PayloadLen) != len(payload) {
			t.Fatalf(
				"declared payload %d, got %d bytes",
				header.PayloadLen,
				len(payload),
			)
		}
	})
}

func FuzzDecodeMessage(f *testing.F) {
	for _, msg := range testMessages() {
		f.Add(uint8(msg.Type()), msg.MarshalPayload())
	}

	f.Add(uint8(0), []byte{})
	f.Add(uint8(255), bytes.Repeat([]byte{0x01}, 64))

	f.Fuzz(func(t *testing.T, msgType uint8, payload []byte) {
		msg, err := protocol.DecodeMessage(
			protocol.MessageType(msgType),
			payload,
			protocol.Limits{},
		)
		if err != nil {
			return
		}

		if msg == nil {
			t.Fatal("nil message with nil error")
		}

		reencoded := msg.MarshalPayload()

		redecoded, redecodeErr := protocol.DecodeMessage(
			protocol.MessageType(msgType),
			reencoded,
			protocol.Limits{},
		)
		if redecodeErr != nil {
			t.Fatalf("re-decode of re-encoded payload failed: %v", redecodeErr)
		}

		if !bytes.Equal(redecoded.MarshalPayload(), reencoded) {
			t.Fatal("re-encode is not a fixed point")
		}
	})
}

func FuzzDecodeRegister(f *testing.F) {
	registerSeed := &protocol.Register{
		ProtocolVersion: protocol.CurrentVersion,
		ExecutorType:    "report-service",
		InstanceID:      "instance-1",
		Capacity:        testCapacity,
		Functions:       []protocol.FunctionSpec{testFunctionSpec()},
	}
	f.Add(registerSeed.MarshalPayload())
	f.Add([]byte{})

	f.Fuzz(func(_ *testing.T, payload []byte) {
		var msg protocol.Register

		_ = msg.UnmarshalPayload(payload, protocol.Limits{})
	})
}

func FuzzDecodeExecRequest(f *testing.F) {
	execSeed := &protocol.ExecRequest{
		JobUUID:       testJobUUID,
		ExecutionID:   testExecutionID,
		AttemptID:     testAttemptID,
		Function:      testFunctionSpec(),
		Args:          []byte{1, 2, 3},
		DeliverResult: true,
	}
	f.Add(execSeed.MarshalPayload())
	f.Add([]byte{})

	f.Fuzz(func(_ *testing.T, payload []byte) {
		var msg protocol.ExecRequest

		_ = msg.UnmarshalPayload(payload, protocol.Limits{})
	})
}

func FuzzDecodeJobUpsert(f *testing.F) {
	f.Add((&protocol.JobUpsert{JobUUID: testJobUUID, JobKey: "k"}).MarshalPayload())
	f.Add((&protocol.JobUpsert{
		JobUUID:      testJobUUID,
		JobKey:       "report-daily",
		ExecutorType: "report-service",
		Function:     testFunctionSpec(),
		Args:         []byte{4, 5},
		Schedule: protocol.ScheduleSpec{
			Kind:          protocol.ScheduleKindOneShot,
			StartUnixNano: testScheduledNanos,
		},
		Enabled: true,
		Pin: protocol.PinSpec{
			Label:  testPinLabel,
			Policy: protocol.PinPolicyPreferred,
		},
		ResultMode: protocol.ResultModeDeliver,
	}).MarshalPayload())
	f.Add([]byte{})

	f.Fuzz(func(_ *testing.T, payload []byte) {
		var msg protocol.JobUpsert

		_ = msg.UnmarshalPayload(payload, protocol.Limits{})
	})
}

func FuzzDecodeLabelUpdate(f *testing.F) {
	f.Add((&protocol.LabelUpdate{
		Announce: []protocol.Label{testAnnounceLabel, testPinLabel},
		Withdraw: []protocol.Label{testWithdrawLabel},
	}).MarshalPayload())
	f.Add((&protocol.LabelUpdate{}).MarshalPayload())
	f.Add(binary.BigEndian.AppendUint32(nil, ^uint32(0)))
	f.Add([]byte{})

	f.Fuzz(func(_ *testing.T, payload []byte) {
		var msg protocol.LabelUpdate

		_ = msg.UnmarshalPayload(payload, protocol.Limits{})
	})
}

func FuzzDecodeResultDelivery(f *testing.F) {
	f.Add((&protocol.ResultDelivery{
		JobUUID:     testJobUUID,
		ExecutionID: testExecutionID,
		Success:     true,
		HasValue:    true,
		Result:      []byte{7, 6, 5},
	}).MarshalPayload())
	f.Add((&protocol.ResultDelivery{
		JobUUID: testJobUUID,
		Error: &protocol.WireError{
			Code:    protocol.ErrorCodeResultNotRequested,
			Message: "not requested",
		},
	}).MarshalPayload())
	f.Add([]byte{})

	f.Fuzz(func(_ *testing.T, payload []byte) {
		var msg protocol.ResultDelivery

		_ = msg.UnmarshalPayload(payload, protocol.Limits{})
	})
}
