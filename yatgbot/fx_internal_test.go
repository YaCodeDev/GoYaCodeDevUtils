package yatgbot

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

type lifecycleSpy struct {
	hooks []fx.Hook
}

func (l *lifecycleSpy) Append(hook fx.Hook) {
	l.hooks = append(l.hooks, hook)
}

func TestNewDispatcherWithLifecycle_RegistersHookAndPropagatesInitError(t *testing.T) {
	t.Parallel()

	lc := &lifecycleSpy{}

	dispatcher := newDispatcherWithLifecycle(lc, &Options{})
	require.NotNil(t, dispatcher)
	require.Len(t, lc.hooks, 1)

	hook := lc.hooks[0]
	require.NotNil(t, hook.OnStart)
	require.NotNil(t, hook.OnStop)

	err := hook.OnStart(context.Background())
	assert.Error(t, err)

	err = hook.OnStop(context.Background())
	assert.NoError(t, err)
}

func TestNewDispatcherLifetimeContext_SurvivesFxStartTimeout(t *testing.T) {
	t.Parallel()

	const startTimeout = 30 * time.Millisecond

	var lifetimeCtx context.Context

	app := fx.New(
		fx.NopLogger,
		fx.StartTimeout(startTimeout),
		fx.Invoke(func(lc fx.Lifecycle) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					ctx, cancel := newDispatcherLifetimeContext()
					lifetimeCtx = ctx

					t.Cleanup(cancel)

					return nil
				},
			})
		}),
	)

	// (*fx.App).Start does not itself apply app.StartTimeout(); it only
	// reacts to whatever context the caller passes in. app.Run derives that
	// context exactly this way before calling Start, so this mirrors what a
	// real long-running deployment (main.go calling app.Run) does.
	startCtx, cancelStart := context.WithTimeout(context.Background(), app.StartTimeout())
	defer cancelStart()

	require.NoError(t, app.Start(startCtx))

	t.Cleanup(func() {
		_ = app.Stop(context.Background())
	})

	require.NotNil(t, lifetimeCtx)

	time.Sleep(startTimeout * 3)

	assert.NoError(
		t,
		lifetimeCtx.Err(),
		"dispatcher lifetime context must not be cancelled by fx's StartTimeout while the app is still running",
	)
}
