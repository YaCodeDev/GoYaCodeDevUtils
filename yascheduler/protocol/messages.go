package protocol

import (
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

// Message is one decoded protocol payload. MarshalPayload renders the
// message body without the frame header; UnmarshalPayload parses and
// validates a body against wire limits.
type Message interface {
	Type() MessageType
	MarshalPayload() []byte
	UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error
}

// Register asks the scheduler to admit this executor connection.
type Register struct {
	ProtocolVersion uint8
	ExecutorType    ExecutorType
	InstanceID      InstanceID
	Capacity        uint32
	Functions       []FunctionSpec
}

// Type implements Message.
func (m *Register) Type() MessageType { return MessageTypeRegister }

// MarshalPayload implements Message.
func (m *Register) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeUint8(m.ProtocolVersion)
	w.writeString(string(m.ExecutorType))
	w.writeString(string(m.InstanceID))
	w.writeUint32(m.Capacity)
	functionCount := uint32(len(m.Functions)) //nolint:gosec // bounded by EncodeFrame size guard
	w.writeUint32(functionCount)

	for i := range m.Functions {
		encodeFunctionSpec(w, &m.Functions[i])
	}

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *Register) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	var err yaerrors.Error

	if m.ProtocolVersion, err = r.readUint8(); err != nil {
		return err.Wrap(logTag + " register: protocol version")
	}

	executorType, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " register: executor type")
	}

	m.ExecutorType = ExecutorType(executorType)

	instanceID, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " register: instance id")
	}

	m.InstanceID = InstanceID(instanceID)

	if m.Capacity, err = r.readUint32(); err != nil {
		return err.Wrap(logTag + " register: capacity")
	}

	count, err := r.readUint32()
	if err != nil {
		return err.Wrap(logTag + " register: function count")
	}

	if count > r.limits.MaxFunctions {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrTooManyFunctions,
			logTag+" register: function count",
		)
	}

	if int(count) > r.remaining()/minFunctionSpecSize {
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrShortBuffer,
			logTag+" register: function count exceeds payload",
		)
	}

	m.Functions = make([]FunctionSpec, count)

	for i := range m.Functions {
		if err = decodeFunctionSpec(r, &m.Functions[i]); err != nil {
			return err.Wrap(logTag + " register: function spec")
		}
	}

	return r.finish()
}

// RegisterAck answers a Register request. HeartbeatIntervalMillis is the
// heartbeat cadence the scheduler expects from this connection.
type RegisterAck struct {
	Accepted                bool
	HeartbeatIntervalMillis uint32
	Error                   *WireError
}

// Type implements Message.
func (m *RegisterAck) Type() MessageType { return MessageTypeRegisterAck }

// MarshalPayload implements Message.
func (m *RegisterAck) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeBool(m.Accepted)
	w.writeUint32(m.HeartbeatIntervalMillis)
	encodeOptionalWireError(w, m.Error)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *RegisterAck) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	var err yaerrors.Error

	if m.Accepted, err = r.readBool(); err != nil {
		return err.Wrap(logTag + " register ack: accepted")
	}

	if m.HeartbeatIntervalMillis, err = r.readUint32(); err != nil {
		return err.Wrap(logTag + " register ack: heartbeat interval")
	}

	if err = decodeOptionalWireError(r, &m.Error); err != nil {
		return err.Wrap(logTag + " register ack: error")
	}

	return r.finish()
}

// Heartbeat reports executor liveness and current load.
type Heartbeat struct {
	InFlight uint32
}

// Type implements Message.
func (m *Heartbeat) Type() MessageType { return MessageTypeHeartbeat }

// MarshalPayload implements Message.
func (m *Heartbeat) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeUint32(m.InFlight)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *Heartbeat) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	var err yaerrors.Error

	if m.InFlight, err = r.readUint32(); err != nil {
		return err.Wrap(logTag + " heartbeat: in flight")
	}

	return r.finish()
}

// HeartbeatAck answers a Heartbeat.
type HeartbeatAck struct{}

// Type implements Message.
func (m *HeartbeatAck) Type() MessageType { return MessageTypeHeartbeatAck }

// MarshalPayload implements Message.
func (m *HeartbeatAck) MarshalPayload() []byte { return nil }

// UnmarshalPayload implements Message.
func (m *HeartbeatAck) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	return newPayloadReader(payload, limits).finish()
}

// ExecRequest asks an executor to run one attempt of one execution.
type ExecRequest struct {
	JobID             JobID
	ExecutionID       ExecutionID
	AttemptID         AttemptID
	AttemptNumber     uint32
	Function          FunctionSpec
	Args              []byte
	ScheduledUnixNano int64
	TimeoutMillis     uint32
}

// Type implements Message.
func (m *ExecRequest) Type() MessageType { return MessageTypeExecRequest }

// MarshalPayload implements Message.
func (m *ExecRequest) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeUint64(uint64(m.JobID))
	w.writeUint64(uint64(m.ExecutionID))
	w.writeUint64(uint64(m.AttemptID))
	w.writeUint32(m.AttemptNumber)
	encodeFunctionSpec(w, &m.Function)
	w.writeBytes(m.Args)
	w.writeInt64(m.ScheduledUnixNano)
	w.writeUint32(m.TimeoutMillis)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *ExecRequest) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	jobID, err := r.readUint64()
	if err != nil {
		return err.Wrap(logTag + " exec request: job id")
	}

	m.JobID = JobID(jobID)

	if err = decodeExecutionRef(r, &m.ExecutionID, &m.AttemptID); err != nil {
		return err.Wrap(logTag + " exec request: execution ref")
	}

	if m.AttemptNumber, err = r.readUint32(); err != nil {
		return err.Wrap(logTag + " exec request: attempt number")
	}

	if err = decodeFunctionSpec(r, &m.Function); err != nil {
		return err.Wrap(logTag + " exec request: function spec")
	}

	if m.Args, err = r.readBytes(); err != nil {
		return err.Wrap(logTag + " exec request: args")
	}

	if m.ScheduledUnixNano, err = r.readInt64(); err != nil {
		return err.Wrap(logTag + " exec request: scheduled time")
	}

	if m.TimeoutMillis, err = r.readUint32(); err != nil {
		return err.Wrap(logTag + " exec request: timeout")
	}

	return r.finish()
}

// ExecAccept reports whether the executor admitted an execution attempt
// into its local run queue.
type ExecAccept struct {
	ExecutionID ExecutionID
	AttemptID   AttemptID
	Accepted    bool
	Error       *WireError
}

// Type implements Message.
func (m *ExecAccept) Type() MessageType { return MessageTypeExecAccept }

// MarshalPayload implements Message.
func (m *ExecAccept) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeUint64(uint64(m.ExecutionID))
	w.writeUint64(uint64(m.AttemptID))
	w.writeBool(m.Accepted)
	encodeOptionalWireError(w, m.Error)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *ExecAccept) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	err := decodeExecutionRef(r, &m.ExecutionID, &m.AttemptID)
	if err != nil {
		return err.Wrap(logTag + " exec accept: execution ref")
	}

	if m.Accepted, err = r.readBool(); err != nil {
		return err.Wrap(logTag + " exec accept: accepted")
	}

	if err = decodeOptionalWireError(r, &m.Error); err != nil {
		return err.Wrap(logTag + " exec accept: error")
	}

	return r.finish()
}

// ExecResult reports the terminal outcome of one accepted attempt.
type ExecResult struct {
	ExecutionID ExecutionID
	AttemptID   AttemptID
	Success     bool
	Result      []byte
	Error       *WireError
}

// Type implements Message.
func (m *ExecResult) Type() MessageType { return MessageTypeExecResult }

// MarshalPayload implements Message.
func (m *ExecResult) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeUint64(uint64(m.ExecutionID))
	w.writeUint64(uint64(m.AttemptID))
	w.writeBool(m.Success)
	w.writeBytes(m.Result)
	encodeOptionalWireError(w, m.Error)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *ExecResult) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	err := decodeExecutionRef(r, &m.ExecutionID, &m.AttemptID)
	if err != nil {
		return err.Wrap(logTag + " exec result: execution ref")
	}

	if m.Success, err = r.readBool(); err != nil {
		return err.Wrap(logTag + " exec result: success")
	}

	if m.Result, err = r.readBytes(); err != nil {
		return err.Wrap(logTag + " exec result: result")
	}

	if err = decodeOptionalWireError(r, &m.Error); err != nil {
		return err.Wrap(logTag + " exec result: error")
	}

	return r.finish()
}

// Cancel asks an executor to stop one running attempt.
type Cancel struct {
	ExecutionID ExecutionID
	AttemptID   AttemptID
	Reason      string
}

// Type implements Message.
func (m *Cancel) Type() MessageType { return MessageTypeCancel }

// MarshalPayload implements Message.
func (m *Cancel) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeUint64(uint64(m.ExecutionID))
	w.writeUint64(uint64(m.AttemptID))
	w.writeString(m.Reason)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *Cancel) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	err := decodeExecutionRef(r, &m.ExecutionID, &m.AttemptID)
	if err != nil {
		return err.Wrap(logTag + " cancel: execution ref")
	}

	if m.Reason, err = r.readString(); err != nil {
		return err.Wrap(logTag + " cancel: reason")
	}

	return r.finish()
}

// Fault reports a wire-level failure, including unsupported protocol
// versions, before the reporting side closes or degrades the connection.
// It is the "protocol error" message of the wire contract.
type Fault struct {
	Cause WireError
}

// Type implements Message.
func (m *Fault) Type() MessageType { return MessageTypeProtocolError }

// MarshalPayload implements Message.
func (m *Fault) MarshalPayload() []byte {
	w := newPayloadWriter()
	encodeWireError(w, &m.Cause)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *Fault) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	if err := decodeWireError(r, &m.Cause); err != nil {
		return err.Wrap(logTag + " protocol error: cause")
	}

	return r.finish()
}

// JobUpsert creates or updates the job identified by the client-chosen
// JobKey. Upserts with the same JobKey address the same job.
type JobUpsert struct {
	JobKey       string
	ExecutorType ExecutorType
	Function     FunctionSpec
	Args         []byte
	Schedule     ScheduleSpec
	Enabled      bool
	Backfill     BackfillSpec
	Retry        RetrySpec
	Overlap      OverlapPolicy
}

// Type implements Message.
func (m *JobUpsert) Type() MessageType { return MessageTypeJobUpsert }

// MarshalPayload implements Message.
func (m *JobUpsert) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeString(m.JobKey)
	w.writeString(string(m.ExecutorType))
	encodeFunctionSpec(w, &m.Function)
	w.writeBytes(m.Args)
	w.writeUint8(uint8(m.Schedule.Kind))
	w.writeInt64(m.Schedule.StartUnixNano)
	w.writeUint64(m.Schedule.IntervalMillis)
	w.writeBool(m.Enabled)
	w.writeUint8(uint8(m.Backfill.Mode))
	w.writeUint32(m.Backfill.MaxCount)
	w.writeUint64(m.Backfill.MaxAgeMillis)
	w.writeUint8(uint8(m.Retry.Policy))
	w.writeUint32(m.Retry.MaxRetries)
	w.writeUint64(m.Retry.InitialDelayMillis)
	w.writeUint64(m.Retry.MaxDelayMillis)
	w.writeUint64(m.Retry.MultiplierBits)
	w.writeUint8(uint8(m.Overlap))

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *JobUpsert) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	jobKey, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " job upsert: job key")
	}

	m.JobKey = jobKey

	executorType, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " job upsert: executor type")
	}

	m.ExecutorType = ExecutorType(executorType)

	if err = decodeFunctionSpec(r, &m.Function); err != nil {
		return err.Wrap(logTag + " job upsert: function spec")
	}

	if m.Args, err = r.readBytes(); err != nil {
		return err.Wrap(logTag + " job upsert: args")
	}

	if err = decodeScheduleSpec(r, &m.Schedule); err != nil {
		return err.Wrap(logTag + " job upsert: schedule")
	}

	if m.Enabled, err = r.readBool(); err != nil {
		return err.Wrap(logTag + " job upsert: enabled")
	}

	if err = decodeBackfillSpec(r, &m.Backfill); err != nil {
		return err.Wrap(logTag + " job upsert: backfill")
	}

	if err = decodeRetrySpec(r, &m.Retry); err != nil {
		return err.Wrap(logTag + " job upsert: retry")
	}

	overlap, err := r.readUint8()
	if err != nil {
		return err.Wrap(logTag + " job upsert: overlap")
	}

	m.Overlap = OverlapPolicy(overlap)

	return r.finish()
}

// JobUpsertAck answers a JobUpsert with the scheduler-minted JobID.
type JobUpsertAck struct {
	JobKey   string
	JobID    JobID
	Accepted bool
	Error    *WireError
}

// Type implements Message.
func (m *JobUpsertAck) Type() MessageType { return MessageTypeJobUpsertAck }

// MarshalPayload implements Message.
func (m *JobUpsertAck) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeString(m.JobKey)
	w.writeUint64(uint64(m.JobID))
	w.writeBool(m.Accepted)
	encodeOptionalWireError(w, m.Error)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *JobUpsertAck) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	jobKey, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " job upsert ack: job key")
	}

	m.JobKey = jobKey

	jobID, err := r.readUint64()
	if err != nil {
		return err.Wrap(logTag + " job upsert ack: job id")
	}

	m.JobID = JobID(jobID)

	if m.Accepted, err = r.readBool(); err != nil {
		return err.Wrap(logTag + " job upsert ack: accepted")
	}

	if err = decodeOptionalWireError(r, &m.Error); err != nil {
		return err.Wrap(logTag + " job upsert ack: error")
	}

	return r.finish()
}

// Shutdown announces that the sender is draining and will close the
// connection after in-flight work settles.
type Shutdown struct {
	Reason string
}

// Type implements Message.
func (m *Shutdown) Type() MessageType { return MessageTypeShutdown }

// MarshalPayload implements Message.
func (m *Shutdown) MarshalPayload() []byte {
	w := newPayloadWriter()
	w.writeString(m.Reason)

	return w.buf
}

// UnmarshalPayload implements Message.
func (m *Shutdown) UnmarshalPayload(payload []byte, limits Limits) yaerrors.Error {
	r := newPayloadReader(payload, limits)

	reason, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " shutdown: reason")
	}

	m.Reason = reason

	return r.finish()
}

// DecodeMessage decodes payload into the typed message matching t.
func DecodeMessage(t MessageType, payload []byte, limits Limits) (Message, yaerrors.Error) {
	var msg Message

	switch t {
	case MessageTypeRegister:
		msg = &Register{}
	case MessageTypeRegisterAck:
		msg = &RegisterAck{}
	case MessageTypeHeartbeat:
		msg = &Heartbeat{}
	case MessageTypeHeartbeatAck:
		msg = &HeartbeatAck{}
	case MessageTypeExecRequest:
		msg = &ExecRequest{}
	case MessageTypeExecAccept:
		msg = &ExecAccept{}
	case MessageTypeExecResult:
		msg = &ExecResult{}
	case MessageTypeCancel:
		msg = &Cancel{}
	case MessageTypeProtocolError:
		msg = &Fault{}
	case MessageTypeJobUpsert:
		msg = &JobUpsert{}
	case MessageTypeJobUpsertAck:
		msg = &JobUpsertAck{}
	case MessageTypeShutdown:
		msg = &Shutdown{}
	default:
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrUnknownMessageType,
			logTag+" decode message",
		)
	}

	if err := msg.UnmarshalPayload(payload, limits); err != nil {
		return nil, err.Wrap(logTag + " decode message")
	}

	return msg, nil
}

func encodeFunctionSpec(w *payloadWriter, f *FunctionSpec) {
	w.writeString(string(f.Name))
	w.writeString(string(f.Version))
	w.writeString(f.InputSignature)
	w.writeString(f.OutputSignature)
}

func decodeFunctionSpec(r *payloadReader, f *FunctionSpec) yaerrors.Error {
	name, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " function spec: name")
	}

	f.Name = FunctionName(name)

	version, err := r.readString()
	if err != nil {
		return err.Wrap(logTag + " function spec: version")
	}

	f.Version = FunctionVersion(version)

	if f.InputSignature, err = r.readString(); err != nil {
		return err.Wrap(logTag + " function spec: input signature")
	}

	if f.OutputSignature, err = r.readString(); err != nil {
		return err.Wrap(logTag + " function spec: output signature")
	}

	return nil
}

func encodeWireError(w *payloadWriter, e *WireError) {
	w.writeUint16(uint16(e.Code))
	w.writeBool(e.Retryable)
	w.writeString(e.Message)
}

func decodeWireError(r *payloadReader, e *WireError) yaerrors.Error {
	code, err := r.readUint16()
	if err != nil {
		return err.Wrap(logTag + " wire error: code")
	}

	e.Code = ErrorCode(code)

	if e.Retryable, err = r.readBool(); err != nil {
		return err.Wrap(logTag + " wire error: retryable")
	}

	if e.Message, err = r.readString(); err != nil {
		return err.Wrap(logTag + " wire error: message")
	}

	return nil
}

func encodeOptionalWireError(w *payloadWriter, e *WireError) {
	if e == nil {
		w.writeBool(false)

		return
	}

	w.writeBool(true)
	encodeWireError(w, e)
}

func decodeOptionalWireError(r *payloadReader, target **WireError) yaerrors.Error {
	present, err := r.readBool()
	if err != nil {
		return err.Wrap(logTag + " optional wire error: presence")
	}

	if !present {
		*target = nil

		return nil
	}

	var wireError WireError

	if err = decodeWireError(r, &wireError); err != nil {
		return err.Wrap(logTag + " optional wire error: value")
	}

	*target = &wireError

	return nil
}

func decodeExecutionRef(
	r *payloadReader,
	executionID *ExecutionID,
	attemptID *AttemptID,
) yaerrors.Error {
	rawExecutionID, err := r.readUint64()
	if err != nil {
		return err.Wrap(logTag + " execution ref: execution id")
	}

	*executionID = ExecutionID(rawExecutionID)

	rawAttemptID, err := r.readUint64()
	if err != nil {
		return err.Wrap(logTag + " execution ref: attempt id")
	}

	*attemptID = AttemptID(rawAttemptID)

	return nil
}

func decodeScheduleSpec(r *payloadReader, s *ScheduleSpec) yaerrors.Error {
	kind, err := r.readUint8()
	if err != nil {
		return err.Wrap(logTag + " schedule spec: kind")
	}

	s.Kind = ScheduleKind(kind)

	if s.StartUnixNano, err = r.readInt64(); err != nil {
		return err.Wrap(logTag + " schedule spec: start")
	}

	if s.IntervalMillis, err = r.readUint64(); err != nil {
		return err.Wrap(logTag + " schedule spec: interval")
	}

	return nil
}

func decodeBackfillSpec(r *payloadReader, b *BackfillSpec) yaerrors.Error {
	mode, err := r.readUint8()
	if err != nil {
		return err.Wrap(logTag + " backfill spec: mode")
	}

	b.Mode = BackfillMode(mode)

	if b.MaxCount, err = r.readUint32(); err != nil {
		return err.Wrap(logTag + " backfill spec: max count")
	}

	if b.MaxAgeMillis, err = r.readUint64(); err != nil {
		return err.Wrap(logTag + " backfill spec: max age")
	}

	return nil
}

func decodeRetrySpec(r *payloadReader, s *RetrySpec) yaerrors.Error {
	policy, err := r.readUint8()
	if err != nil {
		return err.Wrap(logTag + " retry spec: policy")
	}

	s.Policy = RetryPolicy(policy)

	if s.MaxRetries, err = r.readUint32(); err != nil {
		return err.Wrap(logTag + " retry spec: max retries")
	}

	if s.InitialDelayMillis, err = r.readUint64(); err != nil {
		return err.Wrap(logTag + " retry spec: initial delay")
	}

	if s.MaxDelayMillis, err = r.readUint64(); err != nil {
		return err.Wrap(logTag + " retry spec: max delay")
	}

	if s.MultiplierBits, err = r.readUint64(); err != nil {
		return err.Wrap(logTag + " retry spec: multiplier")
	}

	return nil
}
