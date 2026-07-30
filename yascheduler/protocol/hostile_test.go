package protocol_test

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	hostileDeliveredLen  = 32
	hostilePeerDeadline  = 200 * time.Millisecond
	hostileReadGrace     = 3 * time.Second
	hostileAllocBudget   = 64 << 10
	hostileSpecBudget    = 8 << 10
	hostileFunctionCount = protocol.DefaultMaxFunctions - 1
	hostileHugeFrameSize = 64 << 20
	hostileTypeCeiling   = 255
	hostileClaimFields   = 16
)

// starvedReader answers the frame header, delivers a token amount of
// payload, and then reports an unexpected end of stream. It models a peer
// that declares a huge payload and never sends it.
type starvedReader struct {
	data []byte
	off  int
}

func (r *starvedReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.ErrUnexpectedEOF
	}

	n := copy(p, r.data[r.off:])
	r.off += n

	return n, nil
}

// hostileHeader renders a well-formed header declaring payloadLen bytes.
func hostileHeader(msgType protocol.MessageType, payloadLen uint32) []byte {
	header := make([]byte, protocol.HeaderSize)

	binary.BigEndian.PutUint32(header[0:], protocol.Magic)
	header[4] = protocol.CurrentVersion
	header[5] = uint8(msgType)
	binary.BigEndian.PutUint16(header[6:], 0)
	binary.BigEndian.PutUint64(header[8:], uint64(testCorrelationID))
	binary.BigEndian.PutUint32(header[16:], payloadLen)

	return header
}

// allocatedBytes reports the heap bytes fn allocated. It reads a
// process-wide counter, so its callers must not run in parallel with any
// other test.
func allocatedBytes(fn func()) uint64 {
	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	fn()

	runtime.ReadMemStats(&after)

	return after.TotalAlloc - before.TotalAlloc
}

// TestReadFrameDoesNotTrustDeclaredPayloadLength proves a peer cannot make
// the reader allocate the declared payload size before that payload has
// arrived. The allocation must track the bytes actually delivered, so the
// same token payload costs the same whether the peer claims one megabyte
// or sixty-four.
func TestReadFrameDoesNotTrustDeclaredPayloadLength(t *testing.T) {
	cases := []struct {
		name     string
		declared uint32
		limits   protocol.Limits
	}{
		{
			name:     "default frame limit",
			declared: protocol.DefaultMaxFrameSize,
			limits:   protocol.Limits{},
		},
		{
			name:     "raised frame limit",
			declared: hostileHugeFrameSize,
			limits:   protocol.Limits{MaxFrameSize: hostileHugeFrameSize},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			data := append(
				hostileHeader(protocol.MessageTypeExecRequest, testCase.declared),
				make([]byte, hostileDeliveredLen)...,
			)

			var readErr error

			allocated := allocatedBytes(func() {
				reader := &starvedReader{data: data}

				_, _, err := protocol.ReadFrame(reader, testCase.limits)
				if err != nil {
					readErr = err
				}
			})

			if readErr == nil {
				t.Fatal("starved frame should fail to read")
			}

			if allocated > hostileAllocBudget {
				t.Fatalf(
					"ReadFrame allocated %d bytes after %d delivered bytes "+
						"of a %d byte claim, budget %d: a declared length "+
						"must not drive allocation",
					allocated,
					hostileDeliveredLen,
					testCase.declared,
					hostileAllocBudget,
				)
			}
		})
	}
}

// TestReadFrameStarvedPayloadSurfacesDeadline proves the read deadline the
// service sets on its connections reaches the caller instead of hanging
// inside ReadFrame when a peer declares a payload and sends nothing.
func TestReadFrameStarvedPayloadSurfacesDeadline(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()

	defer func() {
		_ = client.Close()
		_ = server.Close()
	}()

	go func() {
		_, _ = server.Write(
			hostileHeader(protocol.MessageTypeExecRequest, protocol.DefaultMaxFrameSize),
		)
	}()

	if err := client.SetReadDeadline(time.Now().Add(hostilePeerDeadline)); err != nil {
		t.Fatalf("set read deadline failed: %v", err)
	}

	done := make(chan error, 1)

	go func() {
		_, _, err := protocol.ReadFrame(client, protocol.Limits{})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("starved frame should fail to read")
		}
	case <-time.After(hostileReadGrace):
		t.Fatal("ReadFrame ignored the connection read deadline")
	}
}

// TestRegisterRejectsFunctionCountBeyondPayload proves a registration
// declaring nearly the maximum number of function specs inside a payload
// far too small to hold them is rejected without allocating one spec per
// declared entry.
func TestRegisterRejectsFunctionCountBeyondPayload(t *testing.T) {
	payload := make([]byte, 0, protocol.HeaderSize)
	payload = append(payload, protocol.CurrentVersion)
	payload = binary.BigEndian.AppendUint32(payload, 0)
	payload = binary.BigEndian.AppendUint32(payload, 0)
	payload = binary.BigEndian.AppendUint32(payload, testCapacity)
	payload = binary.BigEndian.AppendUint32(payload, hostileFunctionCount)

	var decodeErr error

	allocated := allocatedBytes(func() {
		var register protocol.Register

		if err := register.UnmarshalPayload(payload, protocol.Limits{}); err != nil {
			decodeErr = err
		}
	})

	if decodeErr == nil {
		t.Fatal("register declaring more specs than the payload holds should fail")
	}

	if allocated > hostileSpecBudget {
		t.Fatalf(
			"decoding allocated %d bytes from a %d byte payload claiming %d "+
				"specs, budget %d: a declared count must not drive allocation",
			allocated,
			len(payload),
			hostileFunctionCount,
			hostileSpecBudget,
		)
	}
}

// TestDecodeMessageNestedLengthsStayBounded feeds every message type a
// payload whose length prefixes all claim the maximum, and asserts each
// decode fails cleanly rather than panicking or honouring the claim.
func TestDecodeMessageNestedLengthsStayBounded(t *testing.T) {
	t.Parallel()

	claim := make([]byte, 0, hostileClaimFields*4)
	for range hostileClaimFields {
		claim = binary.BigEndian.AppendUint32(claim, ^uint32(0))
	}

	for msgType := range uint8(hostileTypeCeiling) {
		msg, err := protocol.DecodeMessage(
			protocol.MessageType(msgType),
			claim,
			protocol.Limits{},
		)
		if err == nil && msg == nil {
			t.Fatalf("type %d decoded to a nil message with no error", msgType)
		}
	}
}

// TestReadMessageRejectsHalfClosedFrame proves a peer that closes in the
// middle of a frame surfaces an end-of-stream error rather than a hang.
func TestReadMessageRejectsHalfClosedFrame(t *testing.T) {
	t.Parallel()

	frame := encodeFrameOrFatal(t, &protocol.Heartbeat{InFlight: testInFlight})

	_, _, err := protocol.ReadMessage(
		&starvedReader{data: frame[:len(frame)-1]},
		protocol.Limits{},
	)
	if err == nil {
		t.Fatal("half-closed frame should fail to read")
	}

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("half-closed frame error = %v, want io.ErrUnexpectedEOF", err)
	}
}
