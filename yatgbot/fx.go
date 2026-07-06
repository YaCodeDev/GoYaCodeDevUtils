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
		OnStart: func(_ context.Context) error {
			lifetimeCtx, lifetimeCancel := newDispatcherLifetimeContext()
			cancel = lifetimeCancel

			//nolint:contextcheck // lifetimeCtx intentionally roots in Background, see newDispatcherLifetimeContext
			result, err := InitYaTgBot(lifetimeCtx, options)
			if err != nil {
				cancel()

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

// newDispatcherLifetimeContext creates the context that backs the bot's
// long-running work: BackgroundConnect, the updates manager, and the
// message-queue workers started inside InitYaTgBot. It is deliberately
// derived from context.Background() instead of the fx OnStart hook's
// context, because fx bounds that context by fx.App's StartTimeout
// (15s by default) and cancels it once the timeout elapses regardless
// of whether the app is still running. Using it as the parent here would
// silently stop the bot's background workers shortly after every
// "successful" startup. The returned cancel func must be invoked from
// the corresponding OnStop hook to release the context on shutdown.
func newDispatcherLifetimeContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}
