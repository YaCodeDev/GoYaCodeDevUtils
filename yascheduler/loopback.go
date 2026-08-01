package yascheduler

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

var _ engine.Sender = (*loopback)(nil)

// loopback is the in-process link between one engine and one executor
// runtime: two bounded queues drained by two goroutines, mirroring the
// write loop a connection would have. Dispatch is deliberately not a
// direct call - the engine's dispatch path was written for an async
// socket, and a synchronous hand-off would re-enter engine handlers from
// inside dispatchExecution. A full queue therefore refuses the message,
// which the engine already treats as an infrastructure redispatch.
type loopback struct {
	toExecutor chan protocol.Message
	toEngine   chan protocol.Message

	log yalogger.Logger

	started  atomic.Bool
	closed   atomic.Bool
	stopOnce sync.Once
	stopCh   chan struct{}

	executorDone chan struct{}
	engineDone   chan struct{}
}

// newLoopback builds a stopped-channel loopback with two queues of the
// given size.
func newLoopback(queueSize int, log yalogger.Logger) *loopback {
	return &loopback{
		toExecutor:   make(chan protocol.Message, queueSize),
		toEngine:     make(chan protocol.Message, queueSize),
		log:          log,
		stopCh:       make(chan struct{}),
		executorDone: make(chan struct{}),
		engineDone:   make(chan struct{}),
	}
}

// EnqueueMessage implements engine.Sender: it offers one engine-bound
// dispatch to the executor side, refusing rather than blocking when the
// queue is full.
func (lb *loopback) EnqueueMessage(msg protocol.Message) yaerrors.Error {
	return lb.enqueue(lb.toExecutor, msg)
}

// enqueueToEngine is the executor runtime's sink: it offers one message to
// the engine side, refusing rather than blocking when the queue is full.
func (lb *loopback) enqueueToEngine(msg protocol.Message) yaerrors.Error {
	return lb.enqueue(lb.toEngine, msg)
}

// enqueue offers msg to one queue without ever blocking. A stopped
// loopback refuses instead of closing its queues: a send on a closed
// channel panics, and that panic inside a recovered user function would
// masquerade as a function panic.
func (lb *loopback) enqueue(
	queue chan protocol.Message,
	msg protocol.Message,
) yaerrors.Error {
	if lb.closed.Load() {
		return yaerrors.FromError(
			http.StatusServiceUnavailable,
			ErrLoopbackStopped,
			logTag+" loopback enqueue",
		)
	}

	select {
	case queue <- msg:
		return nil
	default:
		return yaerrors.FromError(
			http.StatusServiceUnavailable,
			ErrOutgoingQueueFull,
			logTag+" loopback enqueue",
		)
	}
}

// CloseConnection implements engine.Sender by stopping the loopback.
func (lb *loopback) CloseConnection() {
	lb.Stop()
}

// start launches the two drain goroutines. The engine-bound drain flushes
// its queue on stop, so results of functions that finished during the
// shutdown drain still settle; the executor-bound drain does not, because
// work not yet handed to the runtime belongs to the redispatch path.
func (lb *loopback) start(
	handleExecutorBound func(msg protocol.Message),
	handleEngineBound func(msg protocol.Message),
) {
	lb.started.Store(true)

	go lb.drain(lb.toExecutor, lb.executorDone, handleExecutorBound, false)
	go lb.drain(lb.toEngine, lb.engineDone, handleEngineBound, true)
}

// drain feeds queued messages to handle until the loopback stops.
func (lb *loopback) drain(
	queue chan protocol.Message,
	done chan struct{},
	handle func(msg protocol.Message),
	flushOnStop bool,
) {
	defer close(done)

	for {
		select {
		case <-lb.stopCh:
			if flushOnStop {
				lb.flush(queue, handle)
			}

			return
		case msg := <-queue:
			handle(msg)
		}
	}
}

// flush hands over the messages still queued at stop time.
func (lb *loopback) flush(
	queue chan protocol.Message,
	handle func(msg protocol.Message),
) {
	for {
		select {
		case msg := <-queue:
			handle(msg)
		default:
			return
		}
	}
}

// Stop refuses further messages, stops both drains, and waits for them.
// The data channels stay open forever; only the stop channel closes.
func (lb *loopback) Stop() {
	lb.stopOnce.Do(func() {
		lb.closed.Store(true)
		close(lb.stopCh)
	})

	if !lb.started.Load() {
		return
	}

	<-lb.executorDone
	<-lb.engineDone
}

// stopped reports whether this loopback refuses messages.
func (lb *loopback) stopped() (isStopped bool) {
	return lb.closed.Load()
}

// routeToExecutor feeds one engine-bound dispatch to the executor runtime,
// the way a client's read loop feeds its connection's inbound frames.
func (l *Local) routeToExecutor(execCtx context.Context, msg protocol.Message) {
	switch m := msg.(type) {
	case *protocol.ExecRequest:
		l.runtime.handleExecRequest(execCtx, m)
	case *protocol.Cancel:
		l.runtime.cancelExecution(m.ExecutionID)
	case *protocol.ResultDelivery:
		l.runtime.handleResultDelivery(m)
	default:
		l.log.Warnf(logTag+" unexpected executor-bound message type %d", msg.Type())
	}
}

// routeToEngine feeds one executor-produced message to the engine handler
// that answers it on an authenticated connection.
func (l *Local) routeToEngine(ctx context.Context, msg protocol.Message) {
	switch m := msg.(type) {
	case *protocol.ExecAccept:
		l.engine.HandleExecAccept(ctx, l.cfg.InstanceID, m)
	case *protocol.ExecResult:
		l.engine.HandleExecResult(ctx, l.cfg.InstanceID, m)
	case *protocol.ResultDeliveryAck:
		l.engine.HandleResultAck(ctx, l.cfg.InstanceID, m)
	case *protocol.Heartbeat:
		if entry, found := l.execRegistry.Get(l.cfg.InstanceID); found {
			entry.Heartbeat(time.Now().UTC())
		}

		l.engine.HandleHeartbeat(ctx, l.cfg.InstanceID)
	default:
		l.log.Warnf(logTag+" unexpected engine-bound message type %d", msg.Type())
	}
}
