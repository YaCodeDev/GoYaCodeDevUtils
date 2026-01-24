package yatgbot

import (
	"context"
	"net/http"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gotd/td/tg"
)

var ErrRouteMismatch = yaerrors.FromString(http.StatusContinue, "route: handler type mismatch")

// HandlerNext is a function that represents the next handler in the middleware chain.
type HandlerNext func(ctx context.Context, handlerData *HandlerData, upd tg.UpdateClass) yaerrors.Error

// HandlerMiddleware is a middleware function that can process an update before or after the main handler.
type HandlerMiddleware func(
	ctx context.Context,
	handlerData *HandlerData,
	upd tg.UpdateClass,
	next HandlerNext,
) yaerrors.Error

// AddMiddleware adds one or more middlewares to the router.
//
// Example usage:
//
// r.AddMiddleware(loggingMiddleware, authMiddleware)
func (r *RouterGroup) AddMiddleware(mw ...HandlerMiddleware) {
	r.middlewares = append(r.middlewares, mw...)
}

// chainMiddleware chains the provided middlewares and returns a single HandlerNext function.
func chainMiddleware(final HandlerNext, middlewares ...HandlerMiddleware) HandlerNext {
	if len(middlewares) == 0 {
		return final
	}

	for _, mw := range middlewares {
		middleware := mw
		next := final

		final = func(ctx context.Context, hd *HandlerData, upd tg.UpdateClass) yaerrors.Error {
			return middleware(ctx, hd, upd, next)
		}
	}

	return final
}

// wrapHandler wraps a specific handler function into a generic HandlerNext function.
func wrapHandler[T tg.UpdateClass](
	h func(context.Context, *HandlerData, T) yaerrors.Error,
) HandlerNext {
	return func(ctx context.Context, handlerData *HandlerData, upd tg.UpdateClass) yaerrors.Error {
		t, ok := upd.(T)
		if !ok {
			return ErrRouteMismatch
		}

		return h(ctx, handlerData, t)
	}
}

// collectMiddlewares collects middlewares from the current router and its parent routers.
func (r *RouterGroup) collectMiddlewares() []HandlerMiddleware {
	if r.parent == nil {
		return r.middlewares
	}

	return append(r.parent.collectMiddlewares(), r.middlewares...)
}
