package router

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

type HandlerNext func(ctx context.Context, handlerData *HandlerData, upd any) yaerrors.Error

type HandlerMiddleware func(ctx context.Context, handlerData *HandlerData, upd any, next HandlerNext) yaerrors.Error

func (r *Router) AddMiddleware(mw ...HandlerMiddleware) {
	r.middlewares = append(r.middlewares, mw...)
}

func chainMiddleware(final HandlerNext, middlewares ...HandlerMiddleware) HandlerNext {
	for _, mw := range middlewares {
		middleware := mw
		next := final

		final = func(ctx context.Context, hd *HandlerData, upd any) yaerrors.Error {
			return middleware(ctx, hd, upd, next)
		}
	}

	return final
}

func wrapHandler[T any](h func(context.Context, *HandlerData, *T) yaerrors.Error) HandlerNext {
	return func(ctx context.Context, handlerData *HandlerData, upd any) yaerrors.Error {
		if t, ok := upd.(*T); ok {
			return h(ctx, handlerData, t)
		}

		return nil
	}
}

func (r *Router) collectMiddlewares() []HandlerMiddleware {
	if r.parent == nil {
		return r.middlewares
	}

	return append(r.parent.collectMiddlewares(), r.middlewares...)
}
