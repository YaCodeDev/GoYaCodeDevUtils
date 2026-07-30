// Package protocol implements the yascheduler binary wire protocol spoken
// between executor services (the yascheduler client library) and the
// standalone yascheduler service over a raw TCP connection.
//
// # Frame layout
//
// Every message travels inside a fixed-header frame. All integers are
// big-endian. The header is exactly HeaderSize bytes:
//
//	offset  size  field
//	0       4     magic (Magic, "YASC")
//	4       1     protocol version
//	5       1     message type
//	6       2     flags (reserved, must be zero in version 1)
//	8       8     correlation ID
//	16      4     payload length
//
// The payload is a message-type-specific sequence of explicitly encoded
// fields (see messages.go). No Go-specific serialization such as gob is
// used, so wire compatibility never depends on the internal representation
// of Go types. Argument and result payloads are opaque byte slices at this
// layer; the client library encodes them with MessagePack.
//
// # Compatibility and versioning rules
//
// Version 1 is strict: a receiver must reject a frame whose version it does
// not support by replying with a ProtocolError carrying
// ErrorCodeUnsupportedVersion and closing the connection. Unknown message
// types are protocol errors as well. Future revisions extend the protocol
// only by adding new message types or by incrementing the version byte;
// existing payload layouts for a given (version, type) pair are frozen
// forever. Reserved flag bits must be zero until a version defines them.
//
// # Framing guarantees
//
// ReadFrame reads through io.ReadFull, so partial TCP reads, multiple
// frames arriving in one segment, and single frames split across many
// segments are all handled correctly. Frame and field sizes are validated
// against Limits before any allocation trusts a length prefix, so a
// malicious or broken peer cannot make the receiver allocate unbounded
// memory or panic.
package protocol

// Limits bounds every length-prefixed value a decoder will accept. A zero
// value in any field falls back to the package default, so the zero Limits
// is fully usable.
type Limits struct {
	// MaxFrameSize caps the payload length declared in a frame header.
	MaxFrameSize uint32

	// MaxStringLen caps every length-prefixed string field.
	MaxStringLen uint32

	// MaxBytesLen caps every length-prefixed opaque byte field, such as
	// serialized arguments and results.
	MaxBytesLen uint32

	// MaxFunctions caps the number of function specs in one registration.
	MaxFunctions uint32
}

// DefaultLimits returns the package default wire limits.
func DefaultLimits() Limits {
	return Limits{
		MaxFrameSize: DefaultMaxFrameSize,
		MaxStringLen: DefaultMaxStringLen,
		MaxBytesLen:  DefaultMaxBytesLen,
		MaxFunctions: DefaultMaxFunctions,
	}
}

// normalized returns a copy of l with every zero field replaced by the
// package default for that field.
func (l Limits) normalized() Limits {
	if l.MaxFrameSize == 0 {
		l.MaxFrameSize = DefaultMaxFrameSize
	}

	if l.MaxStringLen == 0 {
		l.MaxStringLen = DefaultMaxStringLen
	}

	if l.MaxBytesLen == 0 {
		l.MaxBytesLen = DefaultMaxBytesLen
	}

	if l.MaxFunctions == 0 {
		l.MaxFunctions = DefaultMaxFunctions
	}

	return l
}
