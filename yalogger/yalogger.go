package yalogger

import (
	"github.com/google/uuid"
)

// Config defines the configuration options for the logger.
//
// BaseLoggerType: The type of logger to use (e.g., Logrus).
// Level: The minimum log level to output (e.g., Info).
// FullTimestamp: Whether to include the full timestamp in log messages.
// DisableTimestamp: Whether to disable timestamps in log messages.
// TimestampFormat: The format to use for timestamps in log messages.
type Config struct {
	BaseLoggerType   BaseLoggerType
	Level            Level
	FullTimestamp    bool
	DisableTimestamp bool
	TimestampFormat  string
}

// BaseLogger is an interface for creating new Logger instances.
type BaseLogger interface {
	// NewLogger creates a new Logger instance from the base logger.
	//
	// Returns:
	//
	//   - Logger: A new instance of Logger.
	NewLogger() Logger
}

// Logger defines a structured logging interface with support for various log levels,
// formatting, and context-aware logging using key-value fields.
type Logger interface {
	// Info logs a message at the Info level.
	// Used for general operational entries about what's happening inside the application.
	//
	// Example usage:
	//
	//   logger.Info("Application started")
	Info(msg string)

	// Infof logs a formatted message at the Info level.
	// Useful for embedding variable values in log messages.
	//
	// Example usage:
	//
	//   logger.Infof("Listening on port %d", port)
	Infof(format string, args ...any)

	// Trace logs a message at the Trace level (very low-level debugging).
	// Best used for tracking detailed flow or internal logic.
	//
	// Example usage:
	//
	//   logger.Trace("Entered handler function")
	Trace(msg string)

	// Tracef logs a formatted message at the Trace level.
	//
	// Example usage:
	//
	//   logger.Tracef("Payload: %+v", payload)
	Tracef(format string, args ...any)

	// Error logs a message at the Error level.
	// Used to indicate a failure that should be investigated.
	//
	// Example usage:
	//
	//   logger.Error("Database connection failed")
	Error(msg string)

	// Errorf logs a formatted message at the Error level.
	//
	// Example usage:
	//
	//   logger.Errorf("Failed to read file: %s", filename)
	Errorf(format string, args ...any)

	// Warn logs a message at the Warn level.
	// Used for non-critical issues that might cause problems.
	//
	// Example usage:
	//
	//   logger.Warn("Low disk space")
	Warn(msg string)

	// Warnf logs a formatted message at the Warn level.
	//
	// Example usage:
	//
	//   logger.Warnf("Cache miss rate: %.2f%%", rate)
	Warnf(format string, args ...any)

	// Debug logs a message at the Debug level.
	// Useful during development to understand application state.
	//
	// Example usage:
	//
	//   logger.Debug("User object created")
	Debug(msg string)

	// Debugf logs a formatted message at the Debug level.
	//
	// Example usage:
	//
	//   logger.Debugf("Response time: %dms", ms)
	Debugf(format string, args ...any)

	// Fatal logs a message at the Fatal level and may terminate the application.
	//
	// Example usage:
	//
	//   logger.Fatal("Configuration missing. Exiting.")
	Fatal(msg string)

	// Fatalf logs a formatted message at the Fatal level.
	//
	// Example usage:
	//
	//   logger.Fatalf("Cannot load config file: %s", path)
	Fatalf(format string, args ...any)

	// WithField returns a logger instance with a single field added to the context.
	//
	// Example usage:
	//
	//   logger.WithField("user_id", 42)
	WithField(key string, value any) Logger

	// WithFields returns a logger instance with multiple fields added to the context.
	//
	// Example usage:
	//
	//   logger.WithFields(map[string]any{"user_id": 42, "role": "admin"})
	WithFields(fields map[string]any) Logger

	// WithRequestStringID returns a logger with a string request ID in the context.
	// Useful for correlating logs in distributed systems.
	//
	// Example usage:
	//
	//   logger.WithRequestStringID("req-123")
	WithRequestStringID(id string) Logger

	// WithRequestUUID returns a logger with a UUID request ID in the context.
	//
	// Example usage:
	//
	//   logger.WithRequestUUID(uuid.New())
	WithRequestUUID(id uuid.UUID) Logger

	// WithRequestID returns a logger with a numeric request ID.
	//
	// Example usage:
	//
	//   logger.WithRequestID(1001)
	WithRequestID(id uint64) Logger

	// WithRandomRequestID returns a logger with a randomly generated request ID.
	// Useful when no external ID is available.
	//
	// Example usage:
	//
	//   logger.WithRandomRequestID()
	WithRandomRequestID() Logger

	// WithSystemRequestID returns a logger with a system config ID in the context.
	// Helpful when logging events tied to specific system configurations.
	//
	// Example usage:
	//
	//   logger.WithSystemRequestID(3)
	WithSystemRequestID(id uint8) Logger

	// WithUserID returns a logger with a user ID in the context.
	//
	// Example usage:
	//
	//   logger.WithUserID(12345)
	WithUserID(userID uint64) Logger

	// GetFields returns the current log context fields as a map.
	//
	// Example usage:
	//
	//	 fields := logger.Fields()
	GetFields() map[string]any

	// DeleteField removes a field from the current log context.
	//
	// Example usage:
	//
	//   logger.DeleteField("user_id")
	DeleteField(key string)

	// GetField returns the value of a field from the current log context.
	//
	// Example usage:
	//
	//   userID, ok := logger.GetField("user_id").(uint64)
	//   if !ok {
	// 	    // Handle type assertion error
	//   }
	GetField(key string) any
}
