package protocol

import (
	"encoding/binary"
	"io"
	"net/http"
	"slices"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Header is the decoded fixed-size prefix of one frame.
type Header struct {
	Version       uint8
	Type          MessageType
	Flags         uint16
	CorrelationID CorrelationID
	PayloadLen    uint32
}

const (
	headerOffsetMagic         = 0
	headerOffsetVersion       = 4
	headerOffsetType          = 5
	headerOffsetFlags         = 6
	headerOffsetCorrelationID = 8
	headerOffsetPayloadLen    = 16
)

// EncodeFrame renders one complete frame: header plus the marshalled
// payload of msg, tagged with the given correlation ID. It rejects
// payloads above limits.MaxFrameSize, so the sender enforces the same
// bound the receiver will.
func EncodeFrame(
	correlationID CorrelationID,
	msg Message,
	limits Limits,
) ([]byte, yaerrors.Error) {
	limits = limits.normalized()

	payload := msg.MarshalPayload()
	if len(payload) > int(limits.MaxFrameSize) {
		return nil, yaerrors.FromError(
			http.StatusRequestEntityTooLarge,
			ErrFrameTooLarge,
			logTag+" encode frame",
		)
	}

	buf := make([]byte, HeaderSize, HeaderSize+len(payload))

	binary.BigEndian.PutUint32(buf[headerOffsetMagic:], Magic)
	buf[headerOffsetVersion] = CurrentVersion
	buf[headerOffsetType] = uint8(msg.Type())
	binary.BigEndian.PutUint16(buf[headerOffsetFlags:], 0)
	binary.BigEndian.PutUint64(buf[headerOffsetCorrelationID:], uint64(correlationID))
	binary.BigEndian.PutUint32(
		buf[headerOffsetPayloadLen:],
		uint32(len(payload)), //nolint:gosec // bounded by the MaxFrameSize guard above
	)

	return append(buf, payload...), nil
}

// WriteFrame encodes msg and writes the complete frame to w. It relies on
// the io.Writer contract that a short write returns a non-nil error, and
// still guards against a misbehaving writer explicitly.
func WriteFrame(
	w io.Writer,
	correlationID CorrelationID,
	msg Message,
	limits Limits,
) yaerrors.Error {
	frame, err := EncodeFrame(correlationID, msg, limits)
	if err != nil {
		return err.Wrap(logTag + " write frame")
	}

	n, writeErr := w.Write(frame)
	if writeErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			writeErr,
			logTag+" write frame",
		)
	}

	if n != len(frame) {
		return yaerrors.FromError(
			http.StatusBadGateway,
			ErrShortWrite,
			logTag+" write frame",
		)
	}

	return nil
}

// decodeHeader parses and validates the fixed-size header bytes.
func decodeHeader(buf []byte, limits Limits) (Header, yaerrors.Error) {
	if binary.BigEndian.Uint32(buf[headerOffsetMagic:]) != Magic {
		return Header{}, yaerrors.FromError(
			http.StatusBadRequest,
			ErrBadMagic,
			logTag+" decode header",
		)
	}

	header := Header{
		Version:       buf[headerOffsetVersion],
		Type:          MessageType(buf[headerOffsetType]),
		Flags:         binary.BigEndian.Uint16(buf[headerOffsetFlags:]),
		CorrelationID: CorrelationID(binary.BigEndian.Uint64(buf[headerOffsetCorrelationID:])),
		PayloadLen:    binary.BigEndian.Uint32(buf[headerOffsetPayloadLen:]),
	}

	if header.Version != CurrentVersion {
		return header, yaerrors.FromError(
			http.StatusBadRequest,
			ErrUnsupportedVersion,
			logTag+" decode header",
		)
	}

	if header.Flags != 0 {
		return header, yaerrors.FromError(
			http.StatusBadRequest,
			ErrReservedFlags,
			logTag+" decode header",
		)
	}

	if header.PayloadLen > limits.MaxFrameSize {
		return header, yaerrors.FromError(
			http.StatusRequestEntityTooLarge,
			ErrFrameTooLarge,
			logTag+" decode header",
		)
	}

	return header, nil
}

// ReadFrame reads exactly one frame from r. Partial reads, frames split
// across TCP segments, and several frames buffered in one segment are all
// handled through io.ReadFull. The returned payload is validated against
// limits for size only; use DecodeMessage to decode it.
//
// The payload buffer grows with the bytes that actually arrive rather than
// with the length the header declares, so a peer that announces a large
// frame and then sends nothing cannot make the receiver reserve the
// announced size. Peak memory therefore tracks delivered bytes plus one
// payloadReadChunk, not the declared length times the connection count.
func ReadFrame(r io.Reader, limits Limits) (Header, []byte, yaerrors.Error) {
	limits = limits.normalized()

	var headerBuf [HeaderSize]byte

	if _, readErr := io.ReadFull(r, headerBuf[:]); readErr != nil {
		return Header{}, nil, yaerrors.FromError(
			http.StatusBadGateway,
			readErr,
			logTag+" read frame header",
		)
	}

	header, err := decodeHeader(headerBuf[:], limits)
	if err != nil {
		return header, nil, err.Wrap(logTag + " read frame")
	}

	payload, err := readPayload(r, header.PayloadLen)
	if err != nil {
		return header, nil, err.Wrap(logTag + " read frame payload")
	}

	return header, payload, nil
}

// readPayload reads exactly length bytes from r, allocating in bounded
// steps so an unfulfilled length declaration costs at most one chunk of
// memory instead of the whole declared size.
func readPayload(r io.Reader, length uint32) ([]byte, yaerrors.Error) {
	total := int(length)
	if total == 0 {
		return nil, nil
	}

	chunk := int(payloadReadChunk)
	payload := make([]byte, 0, min(total, chunk))

	for len(payload) < total {
		filled := len(payload)
		want := min(total-filled, chunk)

		payload = slices.Grow(payload, want)[:filled+want]

		if _, readErr := io.ReadFull(r, payload[filled:]); readErr != nil {
			return nil, yaerrors.FromError(
				http.StatusBadGateway,
				readErr,
				logTag+" read payload chunk",
			)
		}
	}

	return payload, nil
}

// ReadMessage reads one frame and decodes its payload into a typed
// message.
func ReadMessage(r io.Reader, limits Limits) (Header, Message, yaerrors.Error) {
	header, payload, err := ReadFrame(r, limits)
	if err != nil {
		return header, nil, err.Wrap(logTag + " read message")
	}

	msg, err := DecodeMessage(header.Type, payload, limits)
	if err != nil {
		return header, nil, err.Wrap(logTag + " read message")
	}

	return header, msg, nil
}
