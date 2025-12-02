package yarsa

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
)

// DeterministicReader is a deterministic byte stream (DRBG-like) backed by
// HMAC-SHA256(counter). For a fixed seed, it produces the same sequence of
// bytes on every run. The internal 64-bit counter is encoded in big-endian and
// incremented once per 32-byte block (the size of SHA-256 output).
//
// Security note:
//   - This is a simple construction intended for reproducible randomness in tests
//     or key derivation flows you *fully* control. Do not treat it as a drop-in
//     replacement for a NIST-approved DRBG without careful review.
//   - It is **not** concurrency-safe; use one instance per goroutine if needed.
//
// Usage:
//
//	seed := []byte("my secret seed")
//	r := yarsa.NewDeterministicReader(seed)
//
//	// Read 64 bytes deterministically
//	buf := make([]byte, 64)
//	_, _ = r.Read(buf)
//
//	// Re-create with the same seed -> identical 64 bytes in buf2
//	r2 := yarsa.NewDeterministicReader(seed)
//	buf2 := make([]byte, 64)
//	_, _ = r2.Read(buf2)
//	fmt.Println(bytes.Equal(buf, buf2)) // true
type DeterministicReader struct {
	seed    []byte
	counter uint64
	buf     [32]byte
	pos     int
}

// NewDeterministicReader constructs a new deterministic reader from seed.
// The seed slice is **copied** internally to avoid external mutation effects.
// For the same seed, the produced byte stream is identical across runs.
//
// Example:
//
//	r := yarsa.NewDeterministicReader([]byte("seed"))
//	b := make([]byte, 16)
//	_, _ = r.Read(b) // b now contains first 16 bytes of HMAC-SHA256(seed, ctr=0)
func NewDeterministicReader(seed []byte) *DeterministicReader {
	return &DeterministicReader{
		seed:    append([]byte{}, seed...),
		counter: 0,
	}
}

// Read fills p with deterministic bytes, refilling the internal 32-byte block
// as needed. It returns len(p), nil on success.
//
// Contract:
//   - Always returns exactly len(p) unless an unexpected internal error occurs.
//   - Not concurrency-safe. Use one instance per goroutine if needed.
func (r *DeterministicReader) Read(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		if r.pos >= len(r.buf) {
			r.refill()
		}

		avail := len(r.buf) - r.pos

		toCopy := avail

		need := len(p) - written
		if toCopy > need {
			toCopy = need
		}

		copy(p[written:written+toCopy], r.buf[r.pos:r.pos+toCopy])

		r.pos += toCopy

		written += toCopy
	}

	return written, nil
}

// refill computes the next 32-byte block = HMAC-SHA256(seed, bigEndian(counter))
// and resets the buffer position, then increments the counter.
// Not concurrency-safe.
func (r *DeterministicReader) refill() {
	mac := hmac.New(sha256.New, r.seed)

	var ctrBytes [8]byte
	binary.BigEndian.PutUint64(ctrBytes[:], r.counter)
	mac.Write(ctrBytes[:])

	copy(r.buf[:], mac.Sum(nil))

	r.pos = 0

	r.counter++
}
