package yatgclient

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

func TestBackgroundConnect_StartupFailureReturnsError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int32
	log := yalogger.NewBaseLogger(nil).NewLogger()

	err := backgroundConnect(
		ctx,
		func(_ context.Context, _ func(context.Context) error) error {
			attempts.Add(1)

			return errors.New("startup failed")
		},
		log,
		BackgroundConnectConfig{
			InitialInterval: time.Millisecond,
			Multiplier:      1,
			MaxInterval:     time.Millisecond,
			ResetAfter:      time.Millisecond,
		},
	)
	if err == nil {
		t.Fatal("expected startup error, got nil")
	}

	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly one startup attempt, got %d", got)
	}
}

func TestBackgroundConnect_ReconnectsAfterDisconnect(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := yalogger.NewBaseLogger(nil).NewLogger()
	secondAttempt := make(chan struct{})

	var attempts atomic.Int32

	err := backgroundConnect(
		ctx,
		func(runCtx context.Context, f func(context.Context) error) error {
			attempt := attempts.Add(1)

			if attempt == 1 {
				childCtx, childCancel := context.WithCancel(runCtx)
				result := make(chan error, 1)
				go func() {
					result <- f(childCtx)
				}()

				childCancel()
				<-result

				return errors.New("connection dropped")
			}

			close(secondAttempt)

			return f(runCtx)
		},
		log,
		BackgroundConnectConfig{
			InitialInterval: time.Millisecond,
			Multiplier:      1,
			MaxInterval:     5 * time.Millisecond,
			ResetAfter:      20 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatalf("backgroundConnect() unexpected error = %v", err)
	}

	select {
	case <-secondAttempt:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected reconnect attempt after first disconnect")
	}

	cancel()
}

func TestBackgroundConnect_RetriesFailureBeforeReadyAfterInitialConnection(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := yalogger.NewBaseLogger(nil).NewLogger()
	thirdAttempt := make(chan struct{})

	var attempts atomic.Int32

	err := backgroundConnect(
		ctx,
		func(runCtx context.Context, f func(context.Context) error) error {
			switch attempts.Add(1) {
			case 1:
				childCtx, childCancel := context.WithCancel(runCtx)
				result := make(chan error, 1)
				go func() {
					result <- f(childCtx)
				}()

				childCancel()
				<-result

				return errors.New("connection dropped")
			case 2:
				return errors.New("reconnect failed before ready")
			default:
				close(thirdAttempt)

				return f(runCtx)
			}
		},
		log,
		BackgroundConnectConfig{
			InitialInterval: time.Millisecond,
			Multiplier:      1,
			MaxInterval:     5 * time.Millisecond,
			ResetAfter:      20 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatalf("backgroundConnect() unexpected error = %v", err)
	}

	select {
	case <-thirdAttempt:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected retry after post-start reconnect failed before ready")
	}
}

func TestBackgroundConnect_ReconnectsAfterInternalContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := yalogger.NewBaseLogger(nil).NewLogger()
	secondAttempt := make(chan struct{})

	var attempts atomic.Int32

	err := backgroundConnect(
		ctx,
		func(runCtx context.Context, f func(context.Context) error) error {
			attempt := attempts.Add(1)

			if attempt == 1 {
				childCtx, childCancel := context.WithCancel(runCtx)
				result := make(chan error, 1)
				go func() {
					result <- f(childCtx)
				}()

				childCancel()
				<-result

				return context.Canceled
			}

			close(secondAttempt)

			return f(runCtx)
		},
		log,
		BackgroundConnectConfig{
			InitialInterval: time.Millisecond,
			Multiplier:      1,
			MaxInterval:     5 * time.Millisecond,
			ResetAfter:      20 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatalf("backgroundConnect() unexpected error = %v", err)
	}

	select {
	case <-secondAttempt:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected reconnect attempt after internal context cancellation")
	}

	cancel()
}
