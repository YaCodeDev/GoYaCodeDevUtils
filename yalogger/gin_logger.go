package yalogger

import (
	"fmt"
	"io"
	stdhttp "net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GinLoggerConfig defines the configuration options for the Gin logging helpers.
//
// ContextKey: The Gin context key under which the request Logger is stored.
// Defaults to GinContextLoggerKey when empty.
// DisableRequestContext: Whether to skip attaching the Logger to the request's
// context (ctx.Request.Context()).
type GinLoggerConfig struct {
	ContextKey            string
	DisableRequestContext bool
}

// ginLogLevel represents the severity used when forwarding raw Gin output that
// carries no explicit level prefix.
type ginLogLevel uint8

const (
	ginLogLevelDebug ginLogLevel = iota
	ginLogLevelWarn
	ginLogLevelError
)

// ginLogWriter is an io.Writer that forwards Gin's textual output to a Logger,
// using fallback as the level for lines without an explicit prefix.
type ginLogWriter struct {
	log      Logger
	fallback ginLogLevel
}

func normalizeGinLoggerConfig(config *GinLoggerConfig) GinLoggerConfig {
	if config == nil {
		return GinLoggerConfig{
			ContextKey: GinContextLoggerKey,
		}
	}

	copyConfig := *config
	if copyConfig.ContextKey == "" {
		copyConfig.ContextKey = GinContextLoggerKey
	}

	return copyConfig
}

// ConfigureGinDebugLogging redirects Gin's global debug, route, and error output
// through the given Logger, replacing Gin's default colored console writers.
//
// Parameters:
//
//   - log: the Logger that receives Gin's framework output. If nil, this is a no-op.
//
// Example usage:
//
//	yalogger.ConfigureGinDebugLogging(logger)
//	router := gin.New()
func ConfigureGinDebugLogging(log Logger) {
	if log == nil {
		return
	}

	gin.DisableConsoleColor()

	gin.DefaultWriter = &ginLogWriter{
		log:      log,
		fallback: ginLogLevelDebug,
	}
	gin.DefaultErrorWriter = &ginLogWriter{
		log:      log,
		fallback: ginLogLevelError,
	}
	gin.DebugPrintFunc = func(format string, values ...any) {
		writeGinLogLine(log, fmt.Sprintf(format, values...), ginLogLevelDebug)
	}
	gin.DebugPrintRouteFunc = func(
		httpMethod string,
		absolutePath string,
		handlerName string,
		numHandlers int,
	) {
		log.Debugf(
			"[GIN] ROUTE method=%s path=%s handler=%s handlers=%d",
			httpMethod,
			absolutePath,
			handlerName,
			numHandlers,
		)
	}
}

// SetGinContextLogger stores log in the Gin context and, unless disabled, in the
// request's context so later handlers and middleware can retrieve it.
//
// Parameters:
//
//   - ctx: the Gin context. If nil, this is a no-op.
//   - log: the request-scoped Logger to store. If nil, this is a no-op.
//   - config: the logging configuration. If nil, defaults are used.
//
// Example usage:
//
//	yalogger.SetGinContextLogger(ctx, logger.WithRequestUUID(uuid.New()), nil)
func SetGinContextLogger(ctx *gin.Context, log Logger, config *GinLoggerConfig) {
	if ctx == nil || log == nil {
		return
	}

	cfg := normalizeGinLoggerConfig(config)

	ctx.Set(cfg.ContextKey, log)

	if cfg.DisableRequestContext || ctx.Request == nil {
		return
	}

	ctx.Request = ctx.Request.WithContext(ContextWithLogger(ctx.Request.Context(), log))
}

// GinLoggerFromContext returns the request Logger stored in the Gin context (or the
// request's context) merged with the fallback's fields.
//
// Parameters:
//
//   - ctx: the Gin context. If nil, the fallback is returned.
//   - fallback: the Logger used when the context holds none. May be nil.
//   - config: the logging configuration. If nil, defaults are used.
//
// Returns:
//
//   - Logger: the merged Logger, or the fallback when the context holds none.
//
// Example usage:
//
//	log := yalogger.GinLoggerFromContext(ctx, baseLogger, nil)
func GinLoggerFromContext(
	ctx *gin.Context,
	fallback Logger,
	config *GinLoggerConfig,
) Logger {
	if ctx == nil {
		return fallback
	}

	cfg := normalizeGinLoggerConfig(config)

	value, exists := ctx.Get(cfg.ContextKey)
	if exists {
		contextLog, ok := value.(Logger)
		if ok && contextLog != nil {
			if fallback == nil {
				return contextLog
			}

			return fallback.MergeFields(contextLog)
		}
	}

	if ctx.Request == nil {
		return fallback
	}

	return LoggerFromContext(ctx.Request.Context(), fallback)
}

// GinAccessLogger returns a Gin middleware that logs one compact access line per
// request. The level is chosen from the response status: 5xx logs at Error, 4xx at
// Warn, and everything else at Debug.
//
// Parameters:
//
//   - log: the fallback Logger used when the request carries none.
//   - config: the logging configuration. If nil, defaults are used.
//
// Returns:
//
//   - gin.HandlerFunc: the access-logging middleware.
//
// Example usage:
//
//	router.Use(yalogger.GinAccessLogger(logger, nil))
func GinAccessLogger(log Logger, config *GinLoggerConfig) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()

		ctx.Next()

		requestLog := GinLoggerFromContext(ctx, log, config)

		path := ctx.Request.URL.Path
		if rawQuery := ctx.Request.URL.RawQuery; rawQuery != "" {
			path += "?" + rawQuery
		}

		routePath := ctx.FullPath()
		if routePath == "" {
			routePath = ctx.Request.URL.Path
		}

		bodySize := ctx.Writer.Size()
		if bodySize < 0 {
			bodySize = 0
		}

		message := fmt.Sprintf(
			"[GIN] RUN status=%d latency=%s size=%d ip=%s method=%s path=%s route=%s",
			ctx.Writer.Status(),
			time.Since(start).Truncate(time.Microsecond),
			bodySize,
			ctx.ClientIP(),
			ctx.Request.Method,
			path,
			routePath,
		)

		if errMessage := compactGinLine(ctx.Errors.String()); errMessage != "" {
			message += " err=" + errMessage
		}

		switch status := ctx.Writer.Status(); {
		case status >= stdhttp.StatusInternalServerError:
			requestLog.Error(message)
		case status >= stdhttp.StatusBadRequest:
			requestLog.Warn(message)
		default:
			requestLog.Debug(message)
		}
	}
}

// GinRecovery returns a Gin middleware that recovers from panics, logs them via the
// request Logger at Error level, and aborts with HTTP 500. When Gin runs in debug
// mode the compacted stack trace is included.
//
// Parameters:
//
//   - log: the fallback Logger used when the request carries none.
//   - config: the logging configuration. If nil, defaults are used.
//
// Returns:
//
//   - gin.HandlerFunc: the recovery middleware.
//
// Example usage:
//
//	router.Use(yalogger.GinRecovery(logger, nil))
func GinRecovery(log Logger, config *GinLoggerConfig) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(ctx *gin.Context, err any) {
		requestLog := GinLoggerFromContext(ctx, log, config)
		message := fmt.Sprintf(
			"[GIN] PANIC method=%s path=%s route=%s ip=%s err=%v",
			ctx.Request.Method,
			ctx.Request.URL.Path,
			ctx.FullPath(),
			ctx.ClientIP(),
			err,
		)

		if gin.IsDebugging() {
			message += " stack=" + compactGinLine(string(debug.Stack()))
		}

		requestLog.Error(message)
		ctx.AbortWithStatus(stdhttp.StatusInternalServerError)
	})
}

// Write implements io.Writer by forwarding each newline-separated line of p to the
// underlying Logger. It always reports the full input as written.
func (w *ginLogWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(string(p), "\n") {
		writeGinLogLine(w.log, line, w.fallback)
	}

	return len(p), nil
}

func writeGinLogLine(log Logger, raw string, fallback ginLogLevel) {
	if log == nil {
		return
	}

	line := compactGinLine(strings.TrimSpace(strings.TrimPrefix(raw, "[GIN-debug]")))
	if line == "" {
		return
	}

	level := fallback

	switch {
	case strings.HasPrefix(line, "[WARNING]"):
		level = ginLogLevelWarn
		line = strings.TrimSpace(strings.TrimPrefix(line, "[WARNING]"))
		line = "[GIN] WARN: " + line
	case strings.HasPrefix(line, "[ERROR]"):
		level = ginLogLevelError
		line = strings.TrimSpace(strings.TrimPrefix(line, "[ERROR]"))
		line = "[GIN] ERROR: " + line
	default:
		line = "[GIN] DEBUG: " + line
	}

	switch level {
	case ginLogLevelError:
		log.Error(line)
	case ginLogLevelWarn:
		log.Warn(line)
	case ginLogLevelDebug:
		log.Debug(line)
	}
}

func compactGinLine(line string) string {
	replacer := strings.NewReplacer(
		"\x1b[31m", "",
		"\x1b[0m", "",
	)

	return strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(line))), " ")
}
