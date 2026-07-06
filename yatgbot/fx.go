package yatgbot

import (
	"context"

	"go.uber.org/fx"
)

// ModuleName identifies the Fx module providing a *Dispatcher wired to an
// fx.Lifecycle.
const ModuleName = "yatgbot"

// Module provides a *Dispatcher via InitYaTgBot, deferring construction to
// an fx.Lifecycle OnStart hook since InitYaTgBot needs a live context.
// Consuming apps must additionally provide an Options.
var Module = fx.Module(
	ModuleName,
	fx.Provide(newDispatcherWithLifecycle),
)

func newDispatcherWithLifecycle(lc fx.Lifecycle, options *Options) *Dispatcher {
	dispatcher := &Dispatcher{}

	var cancel context.CancelFunc

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			initCtx, initCancel := context.WithCancel(ctx)
			cancel = initCancel

			result, err := InitYaTgBot(initCtx, options)
			if err != nil {
				initCancel()

				return err
			}

			*dispatcher = result

			return nil
		},
		OnStop: func(_ context.Context) error {
			if cancel != nil {
				cancel()
			}

			return nil
		},
	})

	return dispatcher
}
