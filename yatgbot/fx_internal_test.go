package yatgbot

import (
	"context"
	"testing"

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
