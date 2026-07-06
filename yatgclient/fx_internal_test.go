package yatgclient

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
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

func TestNewClientWithLifecycle_RegistersHookAndPropagatesConnectError(t *testing.T) {
	t.Parallel()

	log := yalogger.NewBaseLogger(nil).NewLogger()
	lc := &lifecycleSpy{}

	client := newClientWithLifecycle(lc, &ClientOptions{}, log)
	require.NotNil(t, client)
	require.Len(t, lc.hooks, 1)

	hook := lc.hooks[0]
	require.NotNil(t, hook.OnStart)
	require.NotNil(t, hook.OnStop)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := hook.OnStart(cancelledCtx)
	assert.Error(t, err)

	err = hook.OnStop(context.Background())
	assert.NoError(t, err)
}
