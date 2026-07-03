package yalogger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormLoggerConfig defines the configuration options for the GORM logger.
//
// Level: The minimum GORM log level to output (e.g., gormlogger.Info).
// SlowThreshold: The elapsed-time threshold above which a query is logged as slow.
// IgnoreRecordNotFound: Whether gorm.ErrRecordNotFound errors are suppressed.
type GormLoggerConfig struct {
	Level                gormlogger.LogLevel
	SlowThreshold        time.Duration
	IgnoreRecordNotFound bool
}

// compactGormLogger is an adapter that implements gormlogger.Interface using a
// Logger. It renders GORM output as compact single-line messages.
type compactGormLogger struct {
	log                  Logger
	level                gormlogger.LogLevel
	slowThreshold        time.Duration
	ignoreRecordNotFound bool
}

// NewGormLogger creates a GORM logger that forwards GORM output to the given Logger.
//
// Parameters:
//
//   - log: the underlying Logger that receives the rendered GORM messages.
//   - config: the logger configuration. If nil, sensible defaults are used
//     (Info level, DefaultGormSlowQueryThreshold, and record-not-found errors ignored).
//
// Returns:
//
//   - gormlogger.Interface: a logger ready to be passed to gorm.Config.
//
// Example usage:
//
//	db, err := gorm.Open(dialector, &gorm.Config{
//	    Logger: yalogger.NewGormLogger(logger, nil),
//	})
func NewGormLogger(log Logger, config *GormLoggerConfig) gormlogger.Interface {
	if config == nil {
		config = &GormLoggerConfig{
			Level:                gormlogger.Info,
			SlowThreshold:        DefaultGormSlowQueryThreshold,
			IgnoreRecordNotFound: true,
		}
	}

	return &compactGormLogger{
		log:                  log,
		level:                config.Level,
		slowThreshold:        config.SlowThreshold,
		ignoreRecordNotFound: config.IgnoreRecordNotFound,
	}
}

// LogMode returns a copy of the logger with the log level set to the given value.
//
// Parameters:
//
//   - level: the new GORM log level.
//
// Returns:
//
//   - gormlogger.Interface: a copy of the logger using the provided level.
func (l *compactGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	copyLogger := *l
	copyLogger.level = level

	return &copyLogger
}

func (l *compactGormLogger) loggerFromContext(ctx context.Context) Logger {
	return LoggerFromContext(ctx, l.log)
}

// Info logs GORM informational output. It is emitted through the underlying Logger
// at Debug level to keep routine query logging quiet.
//
// Parameters:
//
//   - ctx: the request context, used to resolve a request-scoped Logger.
//   - msg: the message format.
//   - args: the arguments for the format.
func (l *compactGormLogger) Info(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Info {
		return
	}

	l.loggerFromContext(ctx).Debugf("[GORM] INFO: %s", compactGormLine(fmt.Sprintf(msg, args...)))
}

// Warn logs GORM warning output through the underlying Logger at Warn level.
//
// Parameters:
//
//   - ctx: the request context, used to resolve a request-scoped Logger.
//   - msg: the message format.
//   - args: the arguments for the format.
func (l *compactGormLogger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Warn {
		return
	}

	l.loggerFromContext(ctx).Warnf("[GORM] WARN: %s", compactGormLine(fmt.Sprintf(msg, args...)))
}

// Error logs GORM error output through the underlying Logger at Error level.
//
// Parameters:
//
//   - ctx: the request context, used to resolve a request-scoped Logger.
//   - msg: the message format.
//   - args: the arguments for the format.
func (l *compactGormLogger) Error(ctx context.Context, msg string, args ...any) {
	if l.level < gormlogger.Error {
		return
	}

	l.loggerFromContext(ctx).Errorf("[GORM] ERROR: %s", compactGormLine(fmt.Sprintf(msg, args...)))
}

// Trace logs the outcome of a single SQL statement. Failed statements are logged at
// Error level, statements slower than SlowThreshold at Warn level, and all others at
// Debug level.
//
// Parameters:
//
//   - ctx: the request context, used to resolve a request-scoped Logger.
//   - begin: the time the statement started, used to compute the elapsed duration.
//   - fc: a callback returning the executed SQL and the number of affected rows.
//   - err: the error returned by the statement, if any.
func (l *compactGormLogger) Trace(
	ctx context.Context,
	begin time.Time,
	fc func() (sql string, rowsAffected int64),
	err error,
) {
	if l.level == gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()
	compactSQL := compactGormLine(sql)
	logger := l.loggerFromContext(ctx)

	switch {
	case err != nil && l.level >= gormlogger.Error &&
		(!l.ignoreRecordNotFound || !errors.Is(err, gorm.ErrRecordNotFound)):
		logger.Errorf(
			"[GORM] SQL ERROR elapsed=%s rows=%d sql=%s err=%v",
			elapsed.Truncate(time.Microsecond),
			rows,
			compactSQL,
			err,
		)
	case l.slowThreshold != 0 &&
		elapsed > l.slowThreshold &&
		l.level >= gormlogger.Warn:
		logger.Warnf(
			"[GORM] SLOW SQL elapsed=%s rows=%d sql=%s",
			elapsed.Truncate(time.Microsecond),
			rows,
			compactSQL,
		)
	case l.level >= gormlogger.Info:
		logger.Debugf(
			"[GORM] RUN elapsed=%s rows=%d sql=%s",
			elapsed.Truncate(time.Microsecond),
			rows,
			compactSQL,
		)
	}
}

func compactGormLine(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}
