package protocol

import (
	"encoding/binary"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// payloadWriter accumulates explicitly encoded big-endian fields. Writes
// never fail; size policy is enforced on the decoding side and by the
// frame-level limit check before sending.
type payloadWriter struct {
	buf []byte
}

func newPayloadWriter() *payloadWriter {
	return &payloadWriter{buf: make([]byte, 0, HeaderSize)}
}

func (w *payloadWriter) writeUint8(v uint8) {
	w.buf = append(w.buf, v)
}

func (w *payloadWriter) writeUint16(v uint16) {
	w.buf = binary.BigEndian.AppendUint16(w.buf, v)
}

func (w *payloadWriter) writeUint32(v uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *payloadWriter) writeUint64(v uint64) {
	w.buf = binary.BigEndian.AppendUint64(w.buf, v)
}

func (w *payloadWriter) writeInt64(v int64) {
	w.buf = binary.BigEndian.AppendUint64(
		w.buf,
		uint64(v), //nolint:gosec // two's complement round-trip
	)
}

func (w *payloadWriter) writeBool(v bool) {
	if v {
		w.writeUint8(boolTrue)

		return
	}

	w.writeUint8(boolFalse)
}

func (w *payloadWriter) writeString(v string) {
	w.writeUint32(uint32(len(v))) //nolint:gosec // bounded by the frame-size guard in EncodeFrame
	w.buf = append(w.buf, v...)
}

func (w *payloadWriter) writeBytes(v []byte) {
	w.writeUint32(uint32(len(v))) //nolint:gosec // bounded by the frame-size guard in EncodeFrame
	w.buf = append(w.buf, v...)
}

// payloadReader decodes explicitly encoded big-endian fields while
// enforcing wire limits. Every method returns a structured error instead
// of panicking, whatever the input bytes contain.
type payloadReader struct {
	buf    []byte
	off    int
	limits Limits
}

func newPayloadReader(payload []byte, limits Limits) *payloadReader {
	return &payloadReader{buf: payload, limits: limits.normalized()}
}

func (r *payloadReader) remaining() int {
	return len(r.buf) - r.off
}

func (r *payloadReader) take(n int) ([]byte, yaerrors.Error) {
	if n < 0 || r.remaining() < n {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrShortBuffer,
			logTag+" take",
		)
	}

	chunk := r.buf[r.off : r.off+n]
	r.off += n

	return chunk, nil
}

func (r *payloadReader) readUint8() (uint8, yaerrors.Error) {
	chunk, err := r.take(1)
	if err != nil {
		return 0, err.Wrap(logTag + " read uint8")
	}

	return chunk[0], nil
}

func (r *payloadReader) readUint16() (uint16, yaerrors.Error) {
	chunk, err := r.take(2) //nolint:mnd // field width in bytes
	if err != nil {
		return 0, err.Wrap(logTag + " read uint16")
	}

	return binary.BigEndian.Uint16(chunk), nil
}

func (r *payloadReader) readUint32() (uint32, yaerrors.Error) {
	chunk, err := r.take(4) //nolint:mnd // field width in bytes
	if err != nil {
		return 0, err.Wrap(logTag + " read uint32")
	}

	return binary.BigEndian.Uint32(chunk), nil
}

func (r *payloadReader) readUint64() (uint64, yaerrors.Error) {
	chunk, err := r.take(8) //nolint:mnd // field width in bytes
	if err != nil {
		return 0, err.Wrap(logTag + " read uint64")
	}

	return binary.BigEndian.Uint64(chunk), nil
}

func (r *payloadReader) readInt64() (int64, yaerrors.Error) {
	v, err := r.readUint64()
	if err != nil {
		return 0, err.Wrap(logTag + " read int64")
	}

	return int64(v), nil //nolint:gosec // two's complement round-trip
}

func (r *payloadReader) readBool() (bool, yaerrors.Error) {
	v, err := r.readUint8()
	if err != nil {
		return false, err.Wrap(logTag + " read bool")
	}

	switch v {
	case boolFalse:
		return false, nil
	case boolTrue:
		return true, nil
	default:
		return false, yaerrors.FromError(
			http.StatusBadRequest,
			ErrInvalidBool,
			logTag+" read bool",
		)
	}
}

func (r *payloadReader) readString() (string, yaerrors.Error) {
	length, err := r.readUint32()
	if err != nil {
		return "", err.Wrap(logTag + " read string length")
	}

	if length > r.limits.MaxStringLen {
		return "", yaerrors.FromError(
			http.StatusBadRequest,
			ErrStringTooLong,
			logTag+" read string",
		)
	}

	chunk, err := r.take(int(length))
	if err != nil {
		return "", err.Wrap(logTag + " read string bytes")
	}

	return string(chunk), nil
}

func (r *payloadReader) readBytes() ([]byte, yaerrors.Error) {
	length, err := r.readUint32()
	if err != nil {
		return nil, err.Wrap(logTag + " read bytes length")
	}

	if length > r.limits.MaxBytesLen {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrBytesTooLong,
			logTag+" read bytes",
		)
	}

	chunk, err := r.take(int(length))
	if err != nil {
		return nil, err.Wrap(logTag + " read bytes payload")
	}

	out := make([]byte, length)
	copy(out, chunk)

	return out, nil
}

func (r *payloadReader) finish() yaerrors.Error {
	if r.remaining() != 0 {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrTrailingBytes,
			logTag+" finish",
		)
	}

	return nil
}
