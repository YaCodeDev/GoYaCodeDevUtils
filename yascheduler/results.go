package yascheduler

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// Result is the delivered outcome of one job execution. HasValue separates
// an execution that produced no value - a Void function, or a failure -
// from one whose value encodes to zero bytes; Cause carries the structured
// execution error when Success is false.
type Result struct {
	JobUUID     protocol.JobUUID
	ExecutionID protocol.ExecutionID
	Success     bool
	HasValue    bool
	Payload     []byte
	Cause       *protocol.WireError
}

// Submission is the handle UpsertJob returns for one submitted job. In
// ResultModeDeliver it owns the registered result waiter: Await blocks for
// the delivered result, and Close releases the waiter for a caller that
// stopped caring. Every Await return path closes the submission, so one
// submission answers at most one result.
type Submission struct {
	JobUUID protocol.JobUUID

	registry *resultRegistry
	results  chan *Result
	closed   atomic.Bool
	once     sync.Once
}

// Await blocks until the job result is delivered or ctx ends. A submission
// made under ResultModeIgnore answers ErrResultNotRequested at once, and
// one already closed answers ErrSubmissionClosed. A buffered result is
// preferred over a simultaneously cancelled context.
func (s *Submission) Await(ctx context.Context) (*Result, yaerrors.Error) {
	if s.results == nil {
		return nil, yaerrors.FromError(
			http.StatusConflict,
			ErrResultNotRequested,
			logTag+" await result",
		)
	}

	if s.closed.Load() {
		return nil, yaerrors.FromError(
			http.StatusConflict,
			ErrSubmissionClosed,
			logTag+" await result",
		)
	}

	select {
	case result := <-s.results:
		s.Close()

		return result, nil
	default:
	}

	select {
	case <-ctx.Done():
		s.Close()

		return nil, yaerrors.FromError(
			http.StatusServiceUnavailable,
			ctx.Err(),
			logTag+" await result",
		)
	case result := <-s.results:
		s.Close()

		return result, nil
	}
}

// Close releases the registered result waiter, so an abandoned submission
// cannot leak its registry entry. It is idempotent and safe on an
// ignore-mode submission, which never registered one.
func (s *Submission) Close() {
	s.once.Do(func() {
		s.closed.Store(true)

		if s.registry != nil {
			s.registry.release(s.JobUUID, s.results)
		}
	})
}

// adopt re-keys this submission onto the job identity the scheduler
// answered with. An upsert of an existing job key keeps that job's stored
// UUID, so results deliver under it rather than under the freshly minted
// one the waiter was registered against. It runs before the submission is
// handed to the caller, so no delivery race can observe the move half done
// on the submission itself; the registry move is atomic under its lock.
func (s *Submission) adopt(jobUUID protocol.JobUUID) {
	if s.JobUUID == jobUUID {
		return
	}

	if s.registry != nil {
		s.registry.rekey(s.JobUUID, jobUUID, s.results)
	}

	s.JobUUID = jobUUID
}

// resultRegistry holds the result waiters of one scheduler runtime, keyed
// by job UUID. Waiters are keyed by job identity rather than correlation,
// so they survive reconnects by design: only correlation-scoped pending
// replies die with a connection.
type resultRegistry struct {
	mu      sync.Mutex
	waiters map[protocol.JobUUID]chan *Result
}

// open registers one result waiter for jobUUID when mode asks for
// delivery, and returns the submission handle either way. The waiter
// channel is buffered to one result, so delivery never blocks on a caller
// that has not reached Await yet.
func (r *resultRegistry) open(
	jobUUID protocol.JobUUID,
	mode protocol.ResultMode,
) *Submission {
	submission := &Submission{JobUUID: jobUUID}

	if mode != protocol.ResultModeDeliver {
		return submission
	}

	waiter := make(chan *Result, 1)

	r.mu.Lock()
	r.waiters[jobUUID] = waiter
	r.mu.Unlock()

	submission.registry = r
	submission.results = waiter

	return submission
}

// deliver offers one delivered result to its registered waiter and reports
// whether one existed. The send never blocks: delivery is at-least-once,
// so a duplicate finds the one-slot buffer full and is discarded, while a
// blocking send would wedge the delivery path on an abandoned wait. The
// entry stays registered until the waiter consumes it and closes.
func (r *resultRegistry) deliver(result *Result) (accepted bool) {
	r.mu.Lock()
	waiter, found := r.waiters[result.JobUUID]
	r.mu.Unlock()

	if !found {
		return false
	}

	select {
	case waiter <- result:
	default:
	}

	return true
}

// release drops the registry entry of one waiter. The channel identity
// guard keeps a stale submission's close from evicting a newer waiter that
// re-registered under the same job identity.
func (r *resultRegistry) release(jobUUID protocol.JobUUID, waiter chan *Result) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.waiters[jobUUID] == waiter {
		delete(r.waiters, jobUUID)
	}
}

// rekey moves one waiter to a new job identity, guarded by channel
// identity like release. A newer waiter already registered under the
// target identity is replaced: the scheduler holds at most one pending
// result per job, and it belongs to the latest submitter.
func (r *resultRegistry) rekey(from, to protocol.JobUUID, waiter chan *Result) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.waiters[from] != waiter {
		return
	}

	delete(r.waiters, from)
	r.waiters[to] = waiter
}

// handleResultDelivery feeds one delivered result to its waiter and
// acknowledges the delivery whether or not one existed: a refused ack
// stops the scheduler redelivering a result nobody awaits any more.
func (r *executorRuntime) handleResultDelivery(delivery *protocol.ResultDelivery) {
	accepted := r.results.deliver(&Result{
		JobUUID:     delivery.JobUUID,
		ExecutionID: delivery.ExecutionID,
		Success:     delivery.Success,
		HasValue:    delivery.HasValue,
		Payload:     delivery.Result,
		Cause:       delivery.Error,
	})

	if !accepted {
		r.log.Warnf(logTag+" result delivery for job %s found no waiter", delivery.JobUUID)
	}

	if err := r.sink(&protocol.ResultDeliveryAck{
		JobUUID:  delivery.JobUUID,
		Accepted: accepted,
	}); err != nil {
		r.log.Warnf(logTag+" result ack enqueue failed: %v", err)
	}
}

// DecodeResult decodes the delivered result value into R with MessagePack.
// A result carrying no value - a Void function, or a failed execution -
// answers ErrResultHasNoValue instead of decoding an absent payload.
func DecodeResult[R any](result *Result) (*R, yaerrors.Error) {
	if result == nil {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			ErrNilResult,
			logTag+" decode result",
		)
	}

	if !result.HasValue {
		return nil, yaerrors.FromError(
			http.StatusUnprocessableEntity,
			ErrResultHasNoValue,
			logTag+" decode result",
		)
	}

	value, err := yaencoding.DecodeMessagePack[R](result.Payload)
	if err != nil {
		return nil, err.Wrap(logTag + " decode result")
	}

	return value, nil
}
