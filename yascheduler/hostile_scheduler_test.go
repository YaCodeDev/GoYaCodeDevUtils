package yascheduler_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

const (
	hostileFunction           protocol.FunctionName = "hostile-target"
	hostileExecutionID        protocol.ExecutionID  = 4242
	hostileFirstAttempt       protocol.AttemptID    = 1
	hostileSecondAttempt      protocol.AttemptID    = 2
	hostileUnknownExecutionID protocol.ExecutionID  = 999999

	hostileReconnectWait = 4 * time.Second
	hostileSettleWait    = 300 * time.Millisecond
	hostileCancelWait    = 3 * time.Second

	// hostileHeartbeatMillis is the largest heartbeat cadence a uint32 can
	// carry, about fifty days. A scheduler that answers with it disables a
	// client's dead-connection detection unless the client clamps it.
	hostileHeartbeatMillis uint32 = ^uint32(0)

	// goroutineTolerance absorbs runtime-owned goroutines that outlive one
	// connection cycle, such as timer and netpoll helpers.
	goroutineTolerance = 4

	hostileCycles            = 3
	hostileArgValue    int64 = 5
	hostileUnknownType uint8 = 200
)

var errHostileCancelled = errors.New("cancelled")

// hostileServer accepts connections and hands each one to a behaviour
// function, recording how many connections the client opened.
type hostileServer struct {
	listener net.Listener
	conns    atomic.Int64
	served   chan struct{}
	handlers sync.WaitGroup
}

func startHostileServer(
	t *testing.T,
	behaviour func(t *testing.T, conn net.Conn, index int64),
) *hostileServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	server := &hostileServer{listener: listener, served: make(chan struct{}, 16)}

	server.handlers.Add(1)

	go func() {
		defer server.handlers.Done()

		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}

			index := server.conns.Add(1)

			server.handlers.Add(1)

			go func() {
				defer server.handlers.Done()
				defer func() { _ = conn.Close() }()

				behaviour(t, conn, index)

				select {
				case server.served <- struct{}{}:
				default:
				}
			}()
		}
	}()

	t.Cleanup(server.close)

	return server
}

// close stops accepting and then joins every handler goroutine still in
// flight. The join is what stops a handler reporting into a *testing.T
// whose test already completed: the test cannot finish while one of its
// connections is still being served.
func (s *hostileServer) close() {
	_ = s.listener.Close()

	s.handlers.Wait()
}

// holdOpen keeps a connection open for wait, returning as soon as the
// test enters teardown, so joining the handlers never waits out a sleep
// whose only purpose was to keep the client connected.
func holdOpen(t *testing.T, wait time.Duration) {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-t.Context().Done():
	}
}

// failHandler reports a failure seen on a handler goroutine unless the
// test already entered teardown. A connection the client tears down as it
// stops fails every pending read on it, and that is teardown noise rather
// than evidence about how the client handled a hostile frame.
func failHandler(t *testing.T, format string, args ...any) {
	t.Helper()

	select {
	case <-t.Context().Done():
		return
	default:
	}

	t.Errorf(format, args...)
}

func (s *hostileServer) addr() string {
	return s.listener.Addr().String()
}

// awaitConnections waits until the client has opened at least want
// connections, proving it reconnected after the hostile server dropped it.
func (s *hostileServer) awaitConnections(t *testing.T, want int64) {
	t.Helper()

	deadline := time.Now().Add(hostileReconnectWait)

	for time.Now().Before(deadline) {
		if s.conns.Load() >= want {
			return
		}

		time.Sleep(pollGrain)
	}

	t.Fatalf("client opened %d connections, want at least %d", s.conns.Load(), want)
}

const pollGrain = 5 * time.Millisecond

// hostileRegistry builds a registry holding one blocking function that
// reports when it starts and when its context is cancelled.
func hostileRegistry(
	t *testing.T,
	started chan struct{},
	cancelled chan struct{},
) *yascheduler.Registry {
	t.Helper()

	registry := yascheduler.NewRegistry()

	err := yascheduler.RegisterFunction(
		registry,
		hostileFunction,
		"",
		func(ctx context.Context, value int64) (int64, error) {
			select {
			case started <- struct{}{}:
			default:
			}

			<-ctx.Done()

			select {
			case cancelled <- struct{}{}:
			default:
			}

			return value, errHostileCancelled
		},
	)
	if err != nil {
		t.Fatalf("function should register: %v", err)
	}

	return registry
}

func hostileClientConfig(address string) *yascheduler.Config {
	return &yascheduler.Config{
		Address:                  address,
		ExecutorType:             testExecutorType,
		Capacity:                 4,
		HeartbeatInterval:        20 * time.Millisecond,
		ReconnectInitialInterval: 10 * time.Millisecond,
		ReconnectMaxInterval:     50 * time.Millisecond,
		DrainTimeout:             200 * time.Millisecond,
		DialTimeout:              500 * time.Millisecond,
	}
}

type hostileClient struct {
	client *yascheduler.Client
	cancel context.CancelFunc
	done   chan struct{}
}

func startHostileClient(
	t *testing.T,
	address string,
	registry *yascheduler.Registry,
) *hostileClient {
	t.Helper()

	if registry == nil {
		registry = yascheduler.NewRegistry()
	}

	client, err := yascheduler.New(hostileClientConfig(address), registry, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		if runErr := client.Run(ctx); runErr != nil {
			t.Errorf("Run returned error: %v", runErr)
		}
	}()

	running := &hostileClient{client: client, cancel: cancel, done: done}

	t.Cleanup(func() { running.stop(t) })

	return running
}

func (c *hostileClient) stop(t *testing.T) {
	t.Helper()

	c.cancel()

	select {
	case <-c.done:
	case <-time.After(testRunStopTimeout):
		t.Fatal("client Run did not stop in time")
	}
}

// readRegisterAndAck consumes the client's Register frame and answers it.
func readRegisterAndAck(t *testing.T, conn net.Conn, heartbeatMillis uint32) {
	t.Helper()

	if err := conn.SetDeadline(time.Now().Add(testReadTimeout)); err != nil {
		failHandler(t, "set deadline failed: %v", err)

		return
	}

	header, msg, err := protocol.ReadMessage(conn, protocol.Limits{})
	if err != nil {
		failHandler(t, "register read failed: %v", err)

		return
	}

	if _, isRegister := msg.(*protocol.Register); !isRegister {
		failHandler(t, "first message type = %T, want *protocol.Register", msg)

		return
	}

	if writeErr := protocol.WriteFrame(conn, header.CorrelationID, &protocol.RegisterAck{
		Accepted:                true,
		HeartbeatIntervalMillis: heartbeatMillis,
	}, protocol.Limits{}); writeErr != nil {
		failHandler(t, "register ack write failed: %v", writeErr)
	}
}

// rawFrame renders a header with an arbitrary type and payload, bypassing
// the encoder so unknown types and lying lengths reach the client.
func rawFrame(msgType uint8, payloadLen uint32, payload []byte) []byte {
	header := make([]byte, protocol.HeaderSize)

	binary.BigEndian.PutUint32(header[0:], protocol.Magic)
	header[4] = protocol.CurrentVersion
	header[5] = msgType
	binary.BigEndian.PutUint16(header[6:], 0)
	binary.BigEndian.PutUint64(header[8:], 1)
	binary.BigEndian.PutUint32(header[16:], payloadLen)

	return append(header, payload...)
}

// TestClientSurvivesHostileSchedulerFrames drives the client against a
// scheduler that answers registration correctly and then abuses the
// connection every way the wire allows. The client must never panic, must
// tear the connection down, and must come back with a fresh registration.
func TestClientSurvivesHostileSchedulerFrames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		abuse func(conn net.Conn)
	}{
		{
			name: "garbage bytes",
			abuse: func(conn net.Conn) {
				_, _ = conn.Write(bytes.Repeat([]byte{0xAB}, 512))
			},
		},
		{
			name: "unknown message type",
			abuse: func(conn net.Conn) {
				_, _ = conn.Write(rawFrame(hostileUnknownType, 0, nil))
			},
		},
		{
			name: "payload length above the frame limit",
			abuse: func(conn net.Conn) {
				_, _ = conn.Write(rawFrame(
					uint8(protocol.MessageTypeExecRequest),
					protocol.DefaultMaxFrameSize+1,
					nil,
				))
			},
		},
		{
			name: "declared payload never delivered",
			abuse: func(conn net.Conn) {
				_, _ = conn.Write(rawFrame(
					uint8(protocol.MessageTypeExecRequest),
					protocol.DefaultMaxFrameSize-1,
					[]byte{1, 2, 3},
				))
			},
		},
		{
			name: "half close mid frame",
			abuse: func(conn net.Conn) {
				frame := rawFrame(uint8(protocol.MessageTypeHeartbeatAck), 0, nil)
				_, _ = conn.Write(frame[:protocol.HeaderSize-2])
			},
		},
		{
			name: "reserved flag bits set",
			abuse: func(conn net.Conn) {
				frame := rawFrame(uint8(protocol.MessageTypeHeartbeatAck), 0, nil)
				binary.BigEndian.PutUint16(frame[6:], 0xFFFF)
				_, _ = conn.Write(frame)
			},
		},
		{
			name: "server to client invalid direction",
			abuse: func(conn net.Conn) {
				_ = protocol.WriteFrame(conn, 1, &protocol.ExecResult{
					ExecutionID: hostileExecutionID,
					AttemptID:   hostileFirstAttempt,
					Success:     true,
				}, protocol.Limits{})
			},
		},
		{
			name: "cancel for an unknown execution",
			abuse: func(conn net.Conn) {
				_ = protocol.WriteFrame(conn, 1, &protocol.Cancel{
					ExecutionID: hostileUnknownExecutionID,
					AttemptID:   hostileFirstAttempt,
					Reason:      "forged",
				}, protocol.Limits{})
			},
		},
		{
			name: "upsert ack for an unknown correlation",
			abuse: func(conn net.Conn) {
				_ = protocol.WriteFrame(conn, 987654, &protocol.JobUpsertAck{
					JobKey:   "never-requested",
					Accepted: true,
				}, protocol.Limits{})
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := startHostileServer(t, func(t *testing.T, conn net.Conn, _ int64) {
				readRegisterAndAck(t, conn, testHeartbeatMilli)
				testCase.abuse(conn)

				holdOpen(t, hostileSettleWait)
			})

			startHostileClient(t, server.addr(), nil)

			server.awaitConnections(t, 2)
		})
	}
}

// TestClientCancelsEveryAttemptOfOneExecution proves a Cancel stops every
// local copy of an execution. A scheduler that redispatches after a lease
// expiry can legitimately place two attempts of the same execution on one
// executor, so tracking a single cancel per execution ID silently strands
// the copy whose cancel was overwritten.
func TestClientCancelsEveryAttemptOfOneExecution(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 4)
	cancelled := make(chan struct{}, 4)
	registry := hostileRegistry(t, started, cancelled)

	args, encodeErr := encodeHostileArgs()
	if encodeErr != nil {
		t.Fatalf("args should encode: %v", encodeErr)
	}

	server := startHostileServer(t, func(t *testing.T, conn net.Conn, index int64) {
		readRegisterAndAck(t, conn, testHeartbeatMilli)

		if index != 1 {
			holdOpen(t, hostileSettleWait)

			return
		}

		for _, attempt := range []protocol.AttemptID{
			hostileFirstAttempt,
			hostileSecondAttempt,
		} {
			if err := protocol.WriteFrame(conn, 1, &protocol.ExecRequest{
				ExecutionID: hostileExecutionID,
				AttemptID:   attempt,
				Function:    protocol.FunctionSpec{Name: hostileFunction},
				Args:        args,
			}, protocol.Limits{}); err != nil {
				failHandler(t, "exec request write failed: %v", err)

				return
			}
		}

		awaitStarts(t, started, 2)

		if err := protocol.WriteFrame(conn, 2, &protocol.Cancel{
			ExecutionID: hostileExecutionID,
			AttemptID:   hostileFirstAttempt,
			Reason:      "scheduler cancelled",
		}, protocol.Limits{}); err != nil {
			failHandler(t, "cancel write failed: %v", err)

			return
		}

		holdOpen(t, hostileCancelWait)
	})

	startHostileClient(t, server.addr(), registry)

	awaitCancels(t, cancelled, 2)
}

func awaitStarts(t *testing.T, started chan struct{}, want int) {
	t.Helper()

	for range want {
		select {
		case <-started:
		case <-time.After(hostileCancelWait):
			failHandler(t, "only %d of %d attempts started", want-len(started), want)

			return
		}
	}
}

func awaitCancels(t *testing.T, cancelled chan struct{}, want int) {
	t.Helper()

	seen := 0

	for seen < want {
		select {
		case <-cancelled:
			seen++
		case <-time.After(hostileCancelWait):
			t.Fatalf(
				"%d of %d running attempts observed the cancel: a Cancel must "+
					"reach every local attempt of the execution",
				seen,
				want,
			)
		}
	}
}

func encodeHostileArgs() ([]byte, error) {
	frame, err := protocol.EncodeFrame(
		1,
		&protocol.JobUpsert{JobKey: "x"},
		protocol.Limits{},
	)
	_ = frame

	if err != nil {
		return nil, err
	}

	return msgpackInt(hostileArgValue), nil
}

// msgpackInt renders a positive fixint, the MessagePack encoding the
// client's argument decoder expects for a small int64.
func msgpackInt(value int64) []byte {
	return []byte{byte(value)}
}

// TestClientRecoversFromRegistrationSilence proves a scheduler that
// accepts the TCP connection and never answers Register trips the client's
// registration deadline and is retried instead of wedging Run.
func TestClientRecoversFromRegistrationSilence(t *testing.T) {
	t.Parallel()

	server := startHostileServer(t, func(t *testing.T, conn net.Conn, _ int64) {
		if err := conn.SetReadDeadline(time.Now().Add(testReadTimeout)); err != nil {
			return
		}

		buf := make([]byte, protocol.HeaderSize)
		_, _ = conn.Read(buf)

		holdOpen(t, hostileReconnectWait)
	})

	startHostileClient(t, server.addr(), nil)

	server.awaitConnections(t, 2)
}

// TestClientRecoversFromRegistrationFault proves a scheduler that answers
// registration with an unsupported-version fault is retried rather than
// crashing or wedging the client.
func TestClientRecoversFromRegistrationFault(t *testing.T) {
	t.Parallel()

	server := startHostileServer(t, func(t *testing.T, conn net.Conn, _ int64) {
		if err := conn.SetDeadline(time.Now().Add(testReadTimeout)); err != nil {
			return
		}

		header, _, err := protocol.ReadMessage(conn, protocol.Limits{})
		if err != nil {
			return
		}

		if writeErr := protocol.WriteFrame(conn, header.CorrelationID, &protocol.Fault{
			Cause: protocol.WireError{
				Code:    protocol.ErrorCodeUnsupportedVersion,
				Message: "version 1 not supported",
			},
		}, protocol.Limits{}); writeErr != nil {
			failHandler(t, "fault write failed: %v", writeErr)
		}

		holdOpen(t, hostileSettleWait)
	})

	startHostileClient(t, server.addr(), nil)

	server.awaitConnections(t, 2)
}

// TestClientClampsHostileHeartbeatInterval proves a scheduler cannot
// disable the client's dead-connection detection by acknowledging
// registration with an absurd heartbeat cadence. The client must still
// notice a silent connection and reconnect.
func TestClientClampsHostileHeartbeatInterval(t *testing.T) {
	t.Parallel()

	server := startHostileServer(t, func(t *testing.T, conn net.Conn, _ int64) {
		readRegisterAndAck(t, conn, hostileHeartbeatMillis)

		holdOpen(t, hostileReconnectWait*2)
	})

	startHostileClient(t, server.addr(), nil)

	server.awaitConnections(t, 2)
}

// TestClientLeaksNoGoroutinesAcrossConnectionCycles runs several full
// connect, serve, drop and reconnect cycles and then stops the client,
// proving no per-connection goroutine outlives its connection.
func TestClientLeaksNoGoroutinesAcrossConnectionCycles(t *testing.T) {
	before := runtime.NumGoroutine()

	server := startHostileServer(t, func(t *testing.T, conn net.Conn, _ int64) {
		readRegisterAndAck(t, conn, testHeartbeatMilli)

		_ = protocol.WriteFrame(conn, 1, &protocol.Cancel{
			ExecutionID: hostileUnknownExecutionID,
			AttemptID:   hostileFirstAttempt,
			Reason:      "churn",
		}, protocol.Limits{})

		holdOpen(t, hostileSettleWait)
	})

	client := startHostileClient(t, server.addr(), nil)

	server.awaitConnections(t, hostileCycles)

	server.close()

	client.stop(t)

	settleGoroutines()

	after := runtime.NumGoroutine()
	if after > before+goroutineTolerance {
		t.Fatalf(
			"goroutines grew from %d to %d across %d connection cycles, "+
				"tolerance %d",
			before,
			after,
			hostileCycles,
			goroutineTolerance,
		)
	}
}

func settleGoroutines() {
	for range 20 {
		runtime.Gosched()
		time.Sleep(pollGrain * 2)
	}
}

// TestClientUpsertWaiterReleasedOnConnectionLoss proves a caller blocked
// on a job upsert is released when the connection dies, so the pending
// waiter map cannot retain entries across reconnects.
func TestClientUpsertWaiterReleasedOnConnectionLoss(t *testing.T) {
	t.Parallel()

	upsertSeen := make(chan struct{}, 1)

	server := startHostileServer(t, func(t *testing.T, conn net.Conn, index int64) {
		readRegisterAndAck(t, conn, testHeartbeatMilli)

		if index != 1 {
			holdOpen(t, hostileReconnectWait)

			return
		}

		for {
			if err := conn.SetReadDeadline(time.Now().Add(testReadTimeout)); err != nil {
				return
			}

			_, msg, err := protocol.ReadMessage(conn, protocol.Limits{})
			if err != nil {
				return
			}

			if _, isUpsert := msg.(*protocol.JobUpsert); isUpsert {
				select {
				case upsertSeen <- struct{}{}:
				default:
				}

				return
			}
		}
	})

	client := startHostileClient(t, server.addr(), nil)

	connectCtx, connectCancel := context.WithTimeout(context.Background(), testReadTimeout)
	defer connectCancel()

	if err := client.client.AwaitConnected(connectCtx); err != nil {
		t.Fatalf("client should connect: %v", err)
	}

	upsertCtx, upsertCancel := context.WithTimeout(context.Background(), hostileReconnectWait)
	defer upsertCancel()

	_, err := client.client.UpsertJob(upsertCtx, &yascheduler.JobSpec{
		Key:      "hostile-key",
		Function: protocol.FunctionSpec{Name: hostileFunction},
		Schedule: protocol.ScheduleSpec{Kind: protocol.ScheduleKindOneShot},
	})
	if err == nil {
		t.Fatal("upsert should fail when the connection dies before the ack")
	}

	select {
	case <-upsertSeen:
	case <-time.After(hostileSettleWait):
		t.Fatal("scheduler never saw the upsert frame")
	}
}
