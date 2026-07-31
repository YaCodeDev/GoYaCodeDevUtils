package protocol

import "errors"

var (
	// ErrShortBuffer reports a payload that ended before a field was
	// fully read.
	ErrShortBuffer = errors.New("payload ended before field was fully read")

	// ErrBadMagic reports a frame that does not start with Magic.
	ErrBadMagic = errors.New("frame does not start with yascheduler magic")

	// ErrUnsupportedVersion reports a frame version this package does not
	// speak.
	ErrUnsupportedVersion = errors.New("unsupported protocol version")

	// ErrReservedFlags reports non-zero reserved header flag bits.
	ErrReservedFlags = errors.New("reserved header flags must be zero")

	// ErrUnknownMessageType reports a message type this package does not
	// know.
	ErrUnknownMessageType = errors.New("unknown message type")

	// ErrFrameTooLarge reports a declared payload length above the
	// configured limit.
	ErrFrameTooLarge = errors.New("frame payload exceeds size limit")

	// ErrStringTooLong reports a string field above the configured limit.
	ErrStringTooLong = errors.New("string field exceeds size limit")

	// ErrBytesTooLong reports a byte field above the configured limit.
	ErrBytesTooLong = errors.New("byte field exceeds size limit")

	// ErrTooManyFunctions reports a registration with more functions than
	// the configured limit.
	ErrTooManyFunctions = errors.New("function list exceeds size limit")

	// ErrLabelTooLong reports a routing label above the configured limit.
	ErrLabelTooLong = errors.New("label exceeds size limit")

	// ErrTooManyLabels reports a label list longer than the configured
	// limit.
	ErrTooManyLabels = errors.New("label list exceeds size limit")

	// ErrResultTooLarge reports a delivered result above the configured
	// limit.
	ErrResultTooLarge = errors.New("result payload exceeds size limit")

	// ErrShortUUID reports a payload that ended before a full job
	// identifier was read.
	ErrShortUUID = errors.New("payload ended before job uuid was fully read")

	// ErrTrailingBytes reports leftover bytes after a payload decoded
	// completely.
	ErrTrailingBytes = errors.New("trailing bytes after message payload")

	// ErrInvalidBool reports a boolean byte that is neither 0 nor 1.
	ErrInvalidBool = errors.New("boolean byte is neither 0 nor 1")

	// ErrShortWrite reports a writer that accepted fewer bytes than the
	// encoded frame contains.
	ErrShortWrite = errors.New("short write while sending frame")
)
