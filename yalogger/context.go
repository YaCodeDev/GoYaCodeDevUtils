package yalogger

import "context"

// contextLoggerKey is the private context key under which a Logger is stored,
// ensuring it never collides with keys set by other packages.
type contextLoggerKey struct{}

// ContextWithLogger stores the given Logger in the context so that downstream code
// can reuse the same structured fields via LoggerFromContext.
//
// Parameters:
//
//   - ctx: the parent context. If nil, a new background context is used.
//   - log: the Logger to store. If nil, the context is returned unchanged.
//
// Returns:
//
//   - context.Context: a child context carrying the Logger.
//
// Example usage:
//
//	ctx = yalogger.ContextWithLogger(ctx, logger.WithRequestID(1001))
func ContextWithLogger(ctx context.Context, log Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if log == nil {
		return ctx
	}

	return context.WithValue(ctx, contextLoggerKey{}, log)
}

// LoggerFromContext returns the Logger stored in the context merged with the
// fallback's fields. Fields from the context Logger take precedence on conflict.
//
// Parameters:
//
//   - ctx: the context to read the Logger from. If nil, the fallback is returned.
//   - fallback: the Logger to use when the context holds none. May be nil.
//
// Returns:
//
//   - Logger: the merged Logger, or the fallback when the context holds none.
//
// Example usage:
//
//	log := yalogger.LoggerFromContext(ctx, baseLogger)
func LoggerFromContext(ctx context.Context, fallback Logger) Logger {
	if ctx == nil {
		return fallback
	}

	contextLog, ok := ctx.Value(contextLoggerKey{}).(Logger)
	if !ok || contextLog == nil {
		return fallback
	}

	if fallback == nil {
		return contextLog
	}

	return fallback.MergeFields(contextLog)
}
