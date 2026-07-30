package protocol_test

import (
	"bytes"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	// fuzzStreamFrameCeiling bounds how many frames one fuzz input may
	// decode, so a crafted stream cannot turn the fuzz worker into an
	// endless loop even if a future change makes ReadMessage able to
	// consume zero bytes without failing.
	fuzzStreamFrameCeiling = 1024

	// fuzzStreamLimits keeps the fuzzer's frames small so the corpus
	// explores framing and payload structure rather than bulk data.
	fuzzStreamMaxFrame  uint32 = 4 << 10
	fuzzStreamMaxString uint32 = 256
	fuzzStreamMaxBytes  uint32 = 1 << 10
	fuzzStreamMaxFuncs  uint32 = 32
)

// fuzzStreamSeeds renders realistic multi-frame streams: every message
// type back to back, and the same stream truncated mid-frame.
func fuzzStreamSeeds() [][]byte {
	var stream []byte

	for _, msg := range testMessages() {
		frame, err := protocol.EncodeFrame(testCorrelationID, msg, protocol.Limits{})
		if err != nil {
			continue
		}

		stream = append(stream, frame...)
	}

	seeds := [][]byte{stream}

	if len(stream) > protocol.HeaderSize {
		seeds = append(seeds, stream[:len(stream)-1])
		seeds = append(seeds, stream[:protocol.HeaderSize+1])
	}

	return seeds
}

// FuzzReadMessageStream drives the whole receive path the way a connection
// does: raw bytes from an untrusted peer feed a ReadMessage loop until the
// stream fails. It covers header framing, payload decoding, and the
// transition between consecutive frames in one target, so a state-machine
// crash that neither FuzzReadFrame nor FuzzDecodeMessage can reach in
// isolation still surfaces here.
func FuzzReadMessageStream(f *testing.F) {
	for _, seed := range fuzzStreamSeeds() {
		f.Add(seed)
	}

	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{0x00}, protocol.HeaderSize*2))

	limits := protocol.Limits{
		MaxFrameSize: fuzzStreamMaxFrame,
		MaxStringLen: fuzzStreamMaxString,
		MaxBytesLen:  fuzzStreamMaxBytes,
		MaxFunctions: fuzzStreamMaxFuncs,
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		reader := bytes.NewReader(data)

		for frames := range fuzzStreamFrameCeiling {
			header, msg, err := protocol.ReadMessage(reader, limits)
			if err != nil {
				return
			}

			if msg == nil {
				t.Fatalf("frame %d decoded to a nil message with no error", frames)
			}

			if msg.Type() != header.Type {
				t.Fatalf(
					"frame %d decoded as type %d under header type %d",
					frames,
					msg.Type(),
					header.Type,
				)
			}

			reencoded := msg.MarshalPayload()

			redecoded, redecodeErr := protocol.DecodeMessage(
				header.Type,
				reencoded,
				limits,
			)
			if redecodeErr != nil {
				t.Fatalf(
					"frame %d re-decode of a re-encoded payload failed: %v",
					frames,
					redecodeErr,
				)
			}

			if !bytes.Equal(redecoded.MarshalPayload(), reencoded) {
				t.Fatalf("frame %d re-encode is not a fixed point", frames)
			}
		}

		t.Fatalf("stream of %d bytes never terminated the read loop", len(data))
	})
}
