package yascheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// cancelToken identifies one invocation of one execution on this process.
type cancelToken uint64

// executorRuntime runs registered functions on behalf of one scheduler,
// whatever transport that scheduler sits behind. It owns admission against
// the capacity slots, the shutdown latch and drain accounting, and the
// per-invocation cancel registry. Every message it produces leaves through
// sink, so the same runtime serves a TCP connection and an in-process
// loopback alike.
type executorRuntime struct {
	registry *Registry
	log      yalogger.Logger

	// sink hands one message towards the scheduler without blocking. A
	// Client points it at its connection queue; a Local points it at the
	// loopback's engine-bound queue.
	sink func(msg protocol.Message) yaerrors.Error

	stopping   atomic.Bool
	invocation atomic.Uint64

	// results holds the result waiters of this runtime, keyed by job UUID.
	// They are independent of any connection: a Client keeps them across
	// reconnects, so only the scheduler-side retention budget bounds how
	// long a caller may await a result.
	results resultRegistry

	execSlots chan struct{}

	mu        sync.Mutex
	cancels   map[protocol.ExecutionID]map[cancelToken]context.CancelFunc
	execCount int
	execIdle  chan struct{}
}

// beginShutdown latches shutdown under the same mutex admission takes, so
// every execution either joined the drain accounting before the latch or
// is refused after it, and none can join while a drain is waiting.
func (r *executorRuntime) beginShutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stopping.Store(true)
}

// beginExecution joins one execution to the drain accounting and reports
// whether it was admitted. A latched shutdown refuses it.
func (r *executorRuntime) beginExecution() (admitted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stopping.Load() {
		return false
	}

	r.execCount++

	return true
}

// endExecution retires one execution and releases a waiting drain once the
// last running function has finished.
func (r *executorRuntime) endExecution() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.execCount--

	if r.execCount == 0 && r.execIdle != nil {
		close(r.execIdle)

		r.execIdle = nil
	}
}

// awaitExecutions waits up to timeout for every running function to finish
// and reports whether the drain completed. It parks on a channel rather
// than a wait group, so a drain that gives up leaves no goroutine waiting
// on state a later execution would reuse.
func (r *executorRuntime) awaitExecutions(timeout time.Duration) (drained bool) {
	r.mu.Lock()

	if r.execCount == 0 {
		r.mu.Unlock()

		return true
	}

	if r.execIdle == nil {
		r.execIdle = make(chan struct{})
	}

	idle := r.execIdle

	r.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-idle:
		return true
	case <-timer.C:
		return false
	}
}

// handleExecRequest admits or rejects one execution request and, when
// admitted, runs it on a tracked goroutine.
func (r *executorRuntime) handleExecRequest(execCtx context.Context, req *protocol.ExecRequest) {
	prepared, found := r.registry.lookup(req.Function.Name, req.Function.Version)
	if !found {
		r.rejectExecution(req, &protocol.WireError{
			Code: protocol.ErrorCodeUnknownFunction,
			Message: fmt.Sprintf(
				"function %q %q not registered",
				req.Function.Name,
				req.Function.Version,
			),
		})

		return
	}

	if !functionCompatible(&req.Function, &prepared.spec) {
		r.rejectExecution(req, &protocol.WireError{
			Code:    protocol.ErrorCodeIncompatibleFunction,
			Message: "function signature mismatch",
		})

		return
	}

	select {
	case r.execSlots <- struct{}{}:
	default:
		r.rejectExecution(req, &protocol.WireError{
			Code:      protocol.ErrorCodeCapacityExhausted,
			Retryable: true,
			Message:   "executor capacity exhausted",
		})

		return
	}

	if !r.beginExecution() {
		<-r.execSlots

		r.rejectExecution(req, &protocol.WireError{
			Code:      protocol.ErrorCodeShuttingDown,
			Retryable: true,
			Message:   "executor is shutting down",
		})

		return
	}

	if err := r.sink(&protocol.ExecAccept{
		ExecutionID: req.ExecutionID,
		AttemptID:   req.AttemptID,
		Accepted:    true,
	}); err != nil {
		r.endExecution()

		<-r.execSlots

		r.log.Warnf(logTag+" exec accept enqueue failed: %v", err)

		return
	}

	var (
		runCtx context.Context
		cancel context.CancelFunc
	)

	if req.TimeoutMillis > 0 {
		timeout := time.Duration(req.TimeoutMillis) * time.Millisecond
		runCtx, cancel = context.WithTimeout(execCtx, timeout)
	} else {
		runCtx, cancel = context.WithCancel(execCtx)
	}

	token := r.trackCancel(req.ExecutionID, cancel)

	go r.runExecution(runCtx, cancel, token, prepared, req)
}

// rejectExecution reports a refused execution request.
func (r *executorRuntime) rejectExecution(req *protocol.ExecRequest, cause *protocol.WireError) {
	if err := r.sink(&protocol.ExecAccept{
		ExecutionID: req.ExecutionID,
		AttemptID:   req.AttemptID,
		Accepted:    false,
		Error:       cause,
	}); err != nil {
		r.log.Warnf(logTag+" exec reject enqueue failed: %v", err)
	}
}

// runExecution invokes one accepted execution and reports its result. Its
// context and cancel func are built by the caller, so a Cancel pipelined
// behind the request in the same read buffer already finds the execution
// registered instead of racing this goroutine's own registration.
func (r *executorRuntime) runExecution(
	runCtx context.Context,
	cancel context.CancelFunc,
	token cancelToken,
	prepared *preparedFunction,
	req *protocol.ExecRequest,
) {
	defer r.endExecution()
	defer func() { <-r.execSlots }()
	defer cancel()
	defer r.untrackCancel(req.ExecutionID, token)

	log := r.log.WithFields(map[string]any{
		"job_uuid":     req.JobUUID.String(),
		"execution_id": uint64(req.ExecutionID),
		"attempt_id":   uint64(req.AttemptID),
		"function":     string(req.Function.Name),
		"version":      string(req.Function.Version),
		"attempt":      req.AttemptNumber,
	})

	started := time.Now()
	payload, wireErr := prepared.invoke(runCtx, req.Args)
	elapsed := time.Since(started)

	if wireErr != nil {
		log.Warnf(logTag+" function failed after %s: %s", elapsed, wireErr.Message)
	} else {
		log.Debugf(logTag+" function succeeded after %s", elapsed)
	}

	if err := r.sink(&protocol.ExecResult{
		ExecutionID: req.ExecutionID,
		AttemptID:   req.AttemptID,
		Success:     wireErr == nil,
		HasValue:    payload != nil,
		Result:      payload,
		Error:       wireErr,
	}); err != nil {
		log.Warnf(logTag+" result enqueue failed, scheduler lease will redispatch: %v", err)
	}
}

// trackCancel records one invocation's cancel func under a token minted
// for that invocation alone, and returns the token. Neither the execution
// nor the attempt identifies an invocation on its own: the scheduler may
// have several attempts of one execution in flight here after a
// redispatch, and may repeat an attempt it believes was lost, so a shared
// key would let one invocation's cleanup strand another still running.
func (r *executorRuntime) trackCancel(
	executionID protocol.ExecutionID,
	cancel context.CancelFunc,
) (token cancelToken) {
	token = cancelToken(r.invocation.Add(1))

	r.mu.Lock()
	defer r.mu.Unlock()

	invocations, found := r.cancels[executionID]
	if !found {
		invocations = make(map[cancelToken]context.CancelFunc)
		r.cancels[executionID] = invocations
	}

	invocations[token] = cancel

	return token
}

// untrackCancel drops one invocation and, once an execution has no tracked
// invocations left, drops the execution entry so the map cannot grow with
// finished work.
func (r *executorRuntime) untrackCancel(
	executionID protocol.ExecutionID,
	token cancelToken,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	invocations, found := r.cancels[executionID]
	if !found {
		return
	}

	delete(invocations, token)

	if len(invocations) == 0 {
		delete(r.cancels, executionID)
	}
}

// cancelExecution cancels every invocation of one execution running on
// this process, so a scheduler cancellation stops all of them and not just
// the most recently started one.
func (r *executorRuntime) cancelExecution(executionID protocol.ExecutionID) {
	r.mu.Lock()

	invocations := r.cancels[executionID]
	cancels := make([]context.CancelFunc, 0, len(invocations))

	for _, cancel := range invocations {
		cancels = append(cancels, cancel)
	}

	r.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

// functionCompatible reports whether the requested function matches the
// locally registered spec; empty requested signatures skip that check.
func functionCompatible(requested *protocol.FunctionSpec, local *protocol.FunctionSpec) bool {
	if requested.InputSignature != "" && requested.InputSignature != local.InputSignature {
		return false
	}

	if requested.OutputSignature != "" && requested.OutputSignature != local.OutputSignature {
		return false
	}

	return true
}
