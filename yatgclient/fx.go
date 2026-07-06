package yatgclient

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"go.uber.org/fx"
)

// ModuleName identifies the Fx module providing a *Client wired to an
// fx.Lifecycle.
const ModuleName = "yatgclient"

// Module provides a *Client via NewClient and registers an fx.Lifecycle hook
// that background-connects it on start and cancels the connection on stop.
// Consuming apps must additionally provide a ClientOptions and a
// yalogger.Logger.
var Module = fx.Module(
	ModuleName,
	fx.Provide(newClientWithLifecycle),
)

func newClientWithLifecycle(
	lc fx.Lifecycle,
	options *ClientOptions,
	log yalogger.Logger,
) *Client {
	client := NewClient(options, log)

	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			//nolint:gosec // cancel is stored and invoked from OnStop when the lifecycle stops
			connectCtx, connectCancel := context.WithCancel(ctx)
			cancel = connectCancel

			return client.BackgroundConnect(connectCtx)
		},
		OnStop: func(_ context.Context) error {
			if cancel != nil {
				cancel()
			}

			return nil
		},
	})

	return client
}
