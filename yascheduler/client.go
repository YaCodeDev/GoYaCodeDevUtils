package yascheduler

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabackoff"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// Client maintains one long-lived TCP connection to the yascheduler
// service. It registers this process as an executor, keeps heartbeats
// flowing, receives execution requests, runs the matching registered
// functions, and reports their results. On connection loss it reconnects
// with bounded jittered backoff, registers again under the same process
// instance ID, and stops only when its Run context is cancelled.
type Client struct {
	cfg      Config
	registry *Registry
	log      yalogger.Logger

	running     atomic.Bool
	stopping    atomic.Bool
	correlation atomic.Uint64

	execSlots chan struct{}

	mu        sync.Mutex
	outgoing  chan []byte
	pending   map[protocol.CorrelationID]chan *protocol.JobUpsertAck
	cancels   map[protocol.ExecutionID]map[protocol.AttemptID]context.CancelFunc
	connected chan struct{}
	execCount int
	execIdle  chan struct{}
}

// New builds a Client from cfg and the given function registry. A nil
// log falls back to the base yalogger logger.
func New(cfg *Config, registry *Registry, log yalogger.Logger) (*Client, yaerrors.Error) {
	if registry == nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilRegistry,
			logTag+" new client",
		)
	}

	if cfg == nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNilConfig,
			logTag+" new client",
		)
	}

	normalized, err := cfg.normalized()
	if err != nil {
		return nil, err.Wrap(logTag + " new client")
	}

	if log == nil {
		log = yalogger.NewBaseLogger(nil).NewLogger()
	}

	return &Client{
		cfg:       normalized,
		registry:  registry,
		log:       log.WithField("component", "yascheduler-client"),
		execSlots: make(chan struct{}, normalized.Capacity),
		pending:   make(map[protocol.CorrelationID]chan *protocol.JobUpsertAck),
		cancels:   make(map[protocol.ExecutionID]map[protocol.AttemptID]context.CancelFunc),
		connected: make(chan struct{}),
	}, nil
}

// InstanceID returns the stable process instance ID this client
// registers under.
func (c *Client) InstanceID() protocol.InstanceID {
	return c.cfg.InstanceID
}

// Run connects, registers, and serves until ctx is cancelled. It owns
// the reconnect loop: every connection failure is retried with jittered
// exponential backoff, and a cancelled ctx stops reconnecting, drains
// running functions for up to DrainTimeout, then cancels the leftovers
// and waits one more DrainTimeout for them. A function that ignores its
// context past that answers with ErrDrainTimeout instead of holding Run,
// so a caller's own stop deadline is never pinned by one stuck function.
// Run may be called once at a time per client.
func (c *Client) Run(ctx context.Context) yaerrors.Error {
	if !c.running.CompareAndSwap(false, true) {
		return yaerrors.FromError(
			http.StatusConflict,
			ErrClientAlreadyRunning,
			logTag+" run",
		)
	}

	defer c.running.Store(false)

	c.mu.Lock()
	c.stopping.Store(false)
	c.mu.Unlock()

	execCtx, execCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer execCancel()

	backoff := yabackoff.NewExponential(
		c.cfg.ReconnectInitialInterval,
		DefaultReconnectMultiplier,
		c.cfg.ReconnectMaxInterval,
		0,
	)

	for ctx.Err() == nil {
		if err := c.connectAndServe(ctx, execCtx, &backoff); err != nil {
			if ctx.Err() != nil {
				break
			}

			c.log.Warnf(logTag+" connection lost: %v", err)
		}

		if !c.sleepWithJitter(ctx, backoff.Next()) {
			break
		}
	}

	c.beginShutdown()

	if c.awaitExecutions(c.cfg.DrainTimeout) {
		return nil
	}

	c.log.Warn(logTag + " drain timeout exceeded, cancelling running functions")
	execCancel()

	if !c.awaitExecutions(c.cfg.DrainTimeout) {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			ErrDrainTimeout,
			logTag+" run",
		)
	}

	return nil
}

// connectAndServe performs one dial-register-serve cycle.
func (c *Client) connectAndServe(
	ctx context.Context,
	execCtx context.Context,
	backoff yabackoff.Backoff,
) yaerrors.Error {
	dialer := net.Dialer{Timeout: c.cfg.DialTimeout}

	conn, dialErr := dialer.DialContext(ctx, "tcp", c.cfg.Address)
	if dialErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			dialErr,
			logTag+" dial",
		)
	}

	defer func() { _ = conn.Close() }()

	heartbeatInterval, err := c.register(conn)
	if err != nil {
		return err.Wrap(logTag + " connect")
	}

	backoff.Reset()
	c.log.WithFields(map[string]any{
		"executor_type": string(c.cfg.ExecutorType),
		"instance_id":   string(c.cfg.InstanceID),
	}).Info(logTag + " registered with scheduler")

	return c.serve(ctx, execCtx, conn, heartbeatInterval)
}

// register performs the registration exchange on a fresh connection.
func (c *Client) register(conn net.Conn) (time.Duration, yaerrors.Error) {
	if deadlineErr := conn.SetDeadline(time.Now().Add(c.cfg.DialTimeout)); deadlineErr != nil {
		return 0, yaerrors.FromError(
			http.StatusBadGateway,
			deadlineErr,
			logTag+" register deadline",
		)
	}

	registerMsg := &protocol.Register{
		ProtocolVersion: protocol.CurrentVersion,
		ExecutorType:    c.cfg.ExecutorType,
		InstanceID:      c.cfg.InstanceID,
		Capacity:        c.cfg.Capacity,
		Functions:       c.registry.specs(),
	}

	if err := protocol.WriteFrame(
		conn,
		c.nextCorrelation(),
		registerMsg,
		c.cfg.Limits,
	); err != nil {
		return 0, err.Wrap(logTag + " send register")
	}

	_, msg, err := protocol.ReadMessage(conn, c.cfg.Limits)
	if err != nil {
		return 0, err.Wrap(logTag + " read register ack")
	}

	switch ack := msg.(type) {
	case *protocol.RegisterAck:
		if !ack.Accepted {
			return 0, yaerrors.FromError(
				http.StatusForbidden,
				ErrRegistrationRejected,
				logTag+" register: "+wireErrorText(ack.Error),
			)
		}

		if deadlineErr := conn.SetDeadline(time.Time{}); deadlineErr != nil {
			return 0, yaerrors.FromError(
				http.StatusBadGateway,
				deadlineErr,
				logTag+" clear register deadline",
			)
		}

		return c.resolveHeartbeatInterval(ack.HeartbeatIntervalMillis), nil
	case *protocol.Fault:
		return 0, yaerrors.FromError(
			http.StatusBadGateway,
			ErrRegistrationRejected,
			logTag+" register fault: "+ack.Cause.Message,
		)
	default:
		return 0, yaerrors.FromError(
			http.StatusBadGateway,
			ErrUnexpectedMessage,
			fmt.Sprintf(logTag+" register ack type %d", msg.Type()),
		)
	}
}

// resolveHeartbeatInterval clamps the cadence the scheduler assigned into
// the range this client is willing to honour. A zero value falls back to
// the configured interval; anything shorter than MinHeartbeatInterval or
// longer than both MaxHeartbeatFactor times the configured interval and
// MaxHeartbeatInterval is clamped. The read deadline is a multiple of the
// cadence, so without these bounds a hostile or misconfigured scheduler
// could stretch it far enough to disable dead- and half-open-connection
// detection entirely. The ceiling saturates instead of multiplying, since
// a configured interval of a few decades would otherwise overflow into a
// negative duration and hand a negative cadence to time.NewTicker.
func (c *Client) resolveHeartbeatInterval(assignedMillis uint32) (interval time.Duration) {
	interval = time.Duration(assignedMillis) * time.Millisecond
	if interval <= 0 {
		return c.cfg.HeartbeatInterval
	}

	if interval < MinHeartbeatInterval {
		return MinHeartbeatInterval
	}

	ceiling := MaxHeartbeatInterval
	if c.cfg.HeartbeatInterval < MaxHeartbeatInterval/MaxHeartbeatFactor {
		ceiling = c.cfg.HeartbeatInterval * MaxHeartbeatFactor
	}

	if ceiling < MinHeartbeatInterval {
		ceiling = MinHeartbeatInterval
	}

	if interval > ceiling {
		return ceiling
	}

	return interval
}

// serve owns one registered connection: it starts the write, read, and
// heartbeat loops, and on Run-context cancellation performs the graceful
// path - announce shutdown, drain running functions, close.
func (c *Client) serve(
	ctx context.Context,
	execCtx context.Context,
	conn net.Conn,
	heartbeatInterval time.Duration,
) yaerrors.Error {
	connCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()

	outgoing := make(chan []byte, c.cfg.OutgoingQueueSize)
	c.attachConnection(outgoing)

	defer c.detachConnection()

	errCh := make(chan yaerrors.Error, 3) //nolint:mnd // one slot per loop

	var wg sync.WaitGroup

	wg.Add(3) //nolint:mnd // write, read, heartbeat loops

	writerDone := make(chan struct{})

	go func() {
		defer wg.Done()
		defer close(writerDone)

		errCh <- c.writeLoop(connCtx, conn, outgoing)
	}()

	go func() {
		defer wg.Done()

		errCh <- c.readLoop(connCtx, execCtx, conn, heartbeatInterval)
	}()

	go func() {
		defer wg.Done()

		errCh <- c.heartbeatLoop(connCtx, heartbeatInterval)
	}()

	var firstErr yaerrors.Error

	drained := false

	select {
	case firstErr = <-errCh:
	case <-ctx.Done():
		drained = true

		c.shutdownConnection()
	}

	cancel()

	if drained {
		flushTimer := time.NewTimer(c.cfg.WriteTimeout)
		defer flushTimer.Stop()

		select {
		case <-writerDone:
		case <-flushTimer.C:
		}
	}

	_ = conn.Close()

	wg.Wait()

	return firstErr
}

// shutdownConnection runs the graceful connection path: stop accepting
// new work, announce the shutdown, and let running functions finish for
// up to DrainTimeout so their results still reach the scheduler.
func (c *Client) shutdownConnection() {
	c.beginShutdown()

	if err := c.enqueueMessage(&protocol.Shutdown{Reason: "client shutdown"}); err != nil {
		c.log.Warnf(logTag+" shutdown announce failed: %v", err)
	}

	if !c.awaitExecutions(c.cfg.DrainTimeout) {
		c.log.Warn(logTag + " connection drain timed out with functions still running")
	}
}

// beginShutdown latches shutdown under the same mutex admission takes, so
// every execution either joined the drain accounting before the latch or
// is refused after it, and none can join while a drain is waiting.
func (c *Client) beginShutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopping.Store(true)
}

// beginExecution joins one execution to the drain accounting and reports
// whether it was admitted. A latched shutdown refuses it.
func (c *Client) beginExecution() (admitted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopping.Load() {
		return false
	}

	c.execCount++

	return true
}

// endExecution retires one execution and releases a waiting drain once the
// last running function has finished.
func (c *Client) endExecution() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.execCount--

	if c.execCount == 0 && c.execIdle != nil {
		close(c.execIdle)

		c.execIdle = nil
	}
}

// awaitExecutions waits up to timeout for every running function to finish
// and reports whether the drain completed. It parks on a channel rather
// than a wait group, so a drain that gives up leaves no goroutine waiting
// on state a later execution would reuse.
func (c *Client) awaitExecutions(timeout time.Duration) (drained bool) {
	c.mu.Lock()

	if c.execCount == 0 {
		c.mu.Unlock()

		return true
	}

	if c.execIdle == nil {
		c.execIdle = make(chan struct{})
	}

	idle := c.execIdle

	c.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-idle:
		return true
	case <-timer.C:
		return false
	}
}

// writeLoop owns every write on conn. When the connection context ends
// it flushes the frames still queued, so the graceful shutdown path
// cannot drop result and shutdown frames enqueued before cancellation.
func (c *Client) writeLoop(
	connCtx context.Context,
	conn net.Conn,
	outgoing <-chan []byte,
) yaerrors.Error {
	for {
		select {
		case <-connCtx.Done():
			c.flushOutgoing(conn, outgoing)

			return nil
		case frame := <-outgoing:
			if err := c.writeFrame(conn, frame); err != nil {
				return err
			}
		}
	}
}

// flushOutgoing drains the queued frames onto conn, stopping at the
// first write failure.
func (c *Client) flushOutgoing(conn net.Conn, outgoing <-chan []byte) {
	for {
		select {
		case frame := <-outgoing:
			if err := c.writeFrame(conn, frame); err != nil {
				return
			}
		default:
			return
		}
	}
}

// writeFrame performs one deadline-bounded frame write on conn.
func (c *Client) writeFrame(conn net.Conn, frame []byte) yaerrors.Error {
	deadline := time.Now().Add(c.cfg.WriteTimeout)
	if deadlineErr := conn.SetWriteDeadline(deadline); deadlineErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			deadlineErr,
			logTag+" write deadline",
		)
	}

	if _, writeErr := conn.Write(frame); writeErr != nil {
		return yaerrors.FromError(
			http.StatusBadGateway,
			writeErr,
			logTag+" write frame",
		)
	}

	return nil
}

// readLoop owns every read on conn. The rolling read deadline of three
// heartbeat intervals doubles as dead- and half-open-connection
// detection: a healthy scheduler acknowledges heartbeats, so a silent
// connection times out here and triggers a reconnect.
func (c *Client) readLoop(
	connCtx context.Context,
	execCtx context.Context,
	conn net.Conn,
	heartbeatInterval time.Duration,
) yaerrors.Error {
	readTimeout := heartbeatInterval * readDeadlineMultiplier

	for {
		if connCtx.Err() != nil {
			return nil
		}

		if deadlineErr := conn.SetReadDeadline(time.Now().Add(readTimeout)); deadlineErr != nil {
			return yaerrors.FromError(
				http.StatusBadGateway,
				deadlineErr,
				logTag+" read deadline",
			)
		}

		header, msg, err := protocol.ReadMessage(conn, c.cfg.Limits)
		if err != nil {
			if connCtx.Err() != nil {
				return nil
			}

			return err.Wrap(logTag + " read loop")
		}

		if err = c.handleMessage(execCtx, header, msg); err != nil {
			return err.Wrap(logTag + " read loop")
		}
	}
}

// handleMessage dispatches one inbound frame.
func (c *Client) handleMessage(
	execCtx context.Context,
	header protocol.Header,
	msg protocol.Message,
) yaerrors.Error {
	switch m := msg.(type) {
	case *protocol.ExecRequest:
		c.handleExecRequest(execCtx, m)

		return nil
	case *protocol.HeartbeatAck:
		return nil
	case *protocol.Cancel:
		c.cancelExecution(m.ExecutionID)

		return nil
	case *protocol.JobUpsertAck:
		c.completePending(header.CorrelationID, m)

		return nil
	case *protocol.Fault:
		return yaerrors.FromError(
			http.StatusBadGateway,
			ErrUnexpectedMessage,
			logTag+" scheduler fault: "+m.Cause.Message,
		)
	case *protocol.Shutdown:
		return yaerrors.FromError(
			http.StatusServiceUnavailable,
			ErrServerShutdown,
			logTag+" "+m.Reason,
		)
	default:
		return yaerrors.FromError(
			http.StatusBadRequest,
			ErrUnexpectedMessage,
			fmt.Sprintf(logTag+" inbound type %d", msg.Type()),
		)
	}
}

// handleExecRequest admits or rejects one execution request and, when
// admitted, runs it on a tracked goroutine.
func (c *Client) handleExecRequest(execCtx context.Context, req *protocol.ExecRequest) {
	prepared, found := c.registry.lookup(req.Function.Name, req.Function.Version)
	if !found {
		c.rejectExecution(req, &protocol.WireError{
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
		c.rejectExecution(req, &protocol.WireError{
			Code:    protocol.ErrorCodeIncompatibleFunction,
			Message: "function signature mismatch",
		})

		return
	}

	select {
	case c.execSlots <- struct{}{}:
	default:
		c.rejectExecution(req, &protocol.WireError{
			Code:      protocol.ErrorCodeCapacityExhausted,
			Retryable: true,
			Message:   "executor capacity exhausted",
		})

		return
	}

	if !c.beginExecution() {
		<-c.execSlots

		c.rejectExecution(req, &protocol.WireError{
			Code:      protocol.ErrorCodeShuttingDown,
			Retryable: true,
			Message:   "executor is shutting down",
		})

		return
	}

	if err := c.enqueueMessage(&protocol.ExecAccept{
		ExecutionID: req.ExecutionID,
		AttemptID:   req.AttemptID,
		Accepted:    true,
	}); err != nil {
		c.endExecution()

		<-c.execSlots

		c.log.Warnf(logTag+" exec accept enqueue failed: %v", err)

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

	c.trackCancel(req.ExecutionID, req.AttemptID, cancel)

	go c.runExecution(runCtx, cancel, prepared, req)
}

// rejectExecution reports a refused execution request.
func (c *Client) rejectExecution(req *protocol.ExecRequest, cause *protocol.WireError) {
	if err := c.enqueueMessage(&protocol.ExecAccept{
		ExecutionID: req.ExecutionID,
		AttemptID:   req.AttemptID,
		Accepted:    false,
		Error:       cause,
	}); err != nil {
		c.log.Warnf(logTag+" exec reject enqueue failed: %v", err)
	}
}

// runExecution invokes one accepted execution and reports its result. Its
// context and cancel func are built by the caller, so a Cancel pipelined
// behind the request in the same read buffer already finds the execution
// registered instead of racing this goroutine's own registration.
func (c *Client) runExecution(
	runCtx context.Context,
	cancel context.CancelFunc,
	prepared *preparedFunction,
	req *protocol.ExecRequest,
) {
	defer c.endExecution()
	defer func() { <-c.execSlots }()
	defer cancel()
	defer c.untrackCancel(req.ExecutionID, req.AttemptID)

	log := c.log.WithFields(map[string]any{
		"job_id":       uint64(req.JobID),
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

	if err := c.enqueueMessage(&protocol.ExecResult{
		ExecutionID: req.ExecutionID,
		AttemptID:   req.AttemptID,
		Success:     wireErr == nil,
		Result:      payload,
		Error:       wireErr,
	}); err != nil {
		log.Warnf(logTag+" result enqueue failed, scheduler lease will redispatch: %v", err)
	}
}

// heartbeatLoop reports liveness and current load on a fixed cadence.
func (c *Client) heartbeatLoop(
	connCtx context.Context,
	heartbeatInterval time.Duration,
) yaerrors.Error {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-connCtx.Done():
			return nil
		case <-ticker.C:
			heartbeat := &protocol.Heartbeat{
				InFlight: uint32(len(c.execSlots)), //nolint:gosec // bounded by Capacity
			}
			if err := c.enqueueMessage(heartbeat); err != nil {
				c.log.Warnf(logTag+" heartbeat enqueue failed: %v", err)
			}
		}
	}
}

// enqueueMessage encodes msg and places the frame on the current
// connection's bounded outgoing queue without blocking.
func (c *Client) enqueueMessage(msg protocol.Message) yaerrors.Error {
	return c.enqueueFrame(c.nextCorrelation(), msg)
}

// enqueueFrame encodes msg under an explicit correlation ID and places
// the frame on the current connection's bounded outgoing queue.
func (c *Client) enqueueFrame(
	correlationID protocol.CorrelationID,
	msg protocol.Message,
) yaerrors.Error {
	frame, err := protocol.EncodeFrame(correlationID, msg, c.cfg.Limits)
	if err != nil {
		return err.Wrap(logTag + " enqueue")
	}

	c.mu.Lock()
	outgoing := c.outgoing
	c.mu.Unlock()

	if outgoing == nil {
		return yaerrors.FromError(
			http.StatusServiceUnavailable,
			ErrNotConnected,
			logTag+" enqueue",
		)
	}

	select {
	case outgoing <- frame:
		return nil
	default:
		return yaerrors.FromError(
			http.StatusServiceUnavailable,
			ErrOutgoingQueueFull,
			logTag+" enqueue",
		)
	}
}

// AwaitConnected blocks until the client holds a registered connection
// or ctx ends.
func (c *Client) AwaitConnected(ctx context.Context) yaerrors.Error {
	for {
		c.mu.Lock()
		ready := c.outgoing != nil
		waitCh := c.connected
		c.mu.Unlock()

		if ready {
			return nil
		}

		select {
		case <-ctx.Done():
			return yaerrors.FromError(
				http.StatusServiceUnavailable,
				ctx.Err(),
				logTag+" await connected",
			)
		case <-waitCh:
		}
	}
}

func (c *Client) attachConnection(outgoing chan []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.outgoing = outgoing
	close(c.connected)
}

func (c *Client) detachConnection() {
	c.mu.Lock()

	c.outgoing = nil
	c.connected = make(chan struct{})

	pending := c.pending
	c.pending = make(map[protocol.CorrelationID]chan *protocol.JobUpsertAck)

	c.mu.Unlock()

	for _, waiter := range pending {
		close(waiter)
	}
}

func (c *Client) registerPending(
	correlationID protocol.CorrelationID,
) (chan *protocol.JobUpsertAck, yaerrors.Error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.outgoing == nil {
		return nil, yaerrors.FromError(
			http.StatusServiceUnavailable,
			ErrNotConnected,
			logTag+" register pending",
		)
	}

	waiter := make(chan *protocol.JobUpsertAck, 1)
	c.pending[correlationID] = waiter

	return waiter, nil
}

func (c *Client) unregisterPending(correlationID protocol.CorrelationID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.pending, correlationID)
}

func (c *Client) completePending(
	correlationID protocol.CorrelationID,
	ack *protocol.JobUpsertAck,
) {
	c.mu.Lock()
	waiter, found := c.pending[correlationID]
	delete(c.pending, correlationID)
	c.mu.Unlock()

	if found {
		waiter <- ack
	}
}

// trackCancel records one attempt's cancel func. Attempts are keyed
// individually because the scheduler may have more than one attempt of the
// same execution in flight on this process after a redispatch, and each
// one owns its own context.
func (c *Client) trackCancel(
	executionID protocol.ExecutionID,
	attemptID protocol.AttemptID,
	cancel context.CancelFunc,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	attempts, found := c.cancels[executionID]
	if !found {
		attempts = make(map[protocol.AttemptID]context.CancelFunc)
		c.cancels[executionID] = attempts
	}

	attempts[attemptID] = cancel
}

// untrackCancel drops one attempt and, once an execution has no tracked
// attempts left, drops the execution entry so the map cannot grow with
// finished work.
func (c *Client) untrackCancel(
	executionID protocol.ExecutionID,
	attemptID protocol.AttemptID,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	attempts, found := c.cancels[executionID]
	if !found {
		return
	}

	delete(attempts, attemptID)

	if len(attempts) == 0 {
		delete(c.cancels, executionID)
	}
}

// cancelExecution cancels every attempt of one execution running on this
// process, so a scheduler cancellation stops all of them and not just the
// most recently started one.
func (c *Client) cancelExecution(executionID protocol.ExecutionID) {
	c.mu.Lock()

	attempts := c.cancels[executionID]
	cancels := make([]context.CancelFunc, 0, len(attempts))

	for _, cancel := range attempts {
		cancels = append(cancels, cancel)
	}

	c.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (c *Client) nextCorrelation() protocol.CorrelationID {
	return protocol.CorrelationID(c.correlation.Add(1))
}

// sleepWithJitter waits the given delay scaled by a random factor in
// [jitterMinFactor, 1.0], returning false when ctx ended first.
func (c *Client) sleepWithJitter(ctx context.Context, delay time.Duration) bool {
	factor := jitterMinFactor + rand.Float64()*(1-jitterMinFactor) //nolint:gosec // jitter, not crypto
	jittered := time.Duration(float64(delay) * factor)

	timer := time.NewTimer(jittered)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
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

// wireErrorText renders an optional wire error for logs.
func wireErrorText(wireError *protocol.WireError) string {
	if wireError == nil {
		return "no error detail"
	}

	return wireError.Message
}
