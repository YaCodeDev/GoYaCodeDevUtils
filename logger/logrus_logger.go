package logger

import (
	"math/rand/v2"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// logrusAdapter is an adapter that implements the Logger interface using a logrus.Entry.
// It wraps a logrus.Entry to provide structured logging.
type logrusAdapter struct {
	entry *logrus.Entry
}

// baseLogrus holds a reference to a logrus.Logger instance.
// It serves as the base logger from which new Logger instances can be created.
type baseLogrus struct {
	logger *logrus.Logger
}

// NewBaseLogger creates and configures a new base logger based on the provided configuration.
//
// Returns:
//
//   - BaseLogger: An instance of the base logger for further use.
//
// Notes:
//
//   - If the logger type specified in config is not supported, the function panics.
func NewBaseLogger(config *Config) BaseLogger {
	if config == nil {
		config = &Config{
			BaseLoggerType:   Logrus,
			Level:            DebugLevel,
			FullTimestamp:    false,
			TimestampFormat:  "2006-01-02 15:04:05",
			DisableTimestamp: true,
		}
	}

	switch config.BaseLoggerType {
	case Logrus:
		base := logrus.New()
		base.SetLevel(logrus.Level(config.Level))
		base.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:    config.FullTimestamp,
			TimestampFormat:  config.TimestampFormat,
			DisableTimestamp: config.DisableTimestamp,
		})
	default:
		panic("Unsupported logger type, you are a teapot!!!")
	}

	return &baseLogrus{logger: logrus.New()}
}

// NewLogger creates a new Logger instance from the base logrus logger.
// It wraps the underlying logrus.Logger into a logrusAdapter, which implements the Logger interface.
//
// Returns:
//
//   - Logger: A new instance of logrusAdapter that wraps a new logrus.Entry derived from the base logger.
func (b *baseLogrus) NewLogger() Logger {
	return &logrusAdapter{entry: logrus.NewEntry(b.logger)}
}

// Info logs a message at the Info level.
//
// Parameters:
//
//   - msg: the message to log.
//
// Example usage:
//
//	logger.Info("Application started")
func (l *logrusAdapter) Info(msg string) {
	l.entry.Info(msg)
}

// Infof logs a formatted message at the Info level.
//
// Parameters:
//
//   - format: the message format.
//   - args: the arguments for the format.
//
// Example usage:
//
//	logger.Infof("Server is listening on port %d", port)
func (l *logrusAdapter) Infof(format string, args ...any) {
	l.entry.Infof(format, args...)
}

// Error logs a message at the Error level.
//
// Parameters:
//
//   - msg: the error message.
//
// Example usage:
//
//	logger.Error("Database connection failed")
func (l *logrusAdapter) Error(msg string) {
	l.entry.Error(msg)
}

// Errorf logs a formatted error message at the Error level.
//
// Parameters:
//
//   - format: the message format.
//   - args: the arguments for the format.
//
// Example usage:
//
//	logger.Errorf("Failed to read file: %s", filename)
func (l *logrusAdapter) Errorf(format string, args ...any) {
	l.entry.Errorf(format, args...)
}

// Warn logs a warning message at the Warn level.
//
// Parameters:
//
//   - msg: the warning message.
//
// Example usage:
//
//	logger.Warn("Low disk space")
func (l *logrusAdapter) Warn(msg string) {
	l.entry.Warn(msg)
}

// Warnf logs a formatted warning message at the Warn level.
//
// Parameters:
//
//   - format: the message format.
//   - args: the arguments for the format.
//
// Example usage:
//
//	logger.Warnf("Cache miss rate: %.2f%%", rate)
func (l *logrusAdapter) Warnf(format string, args ...any) {
	l.entry.Warnf(format, args...)
}

// Debug logs a debug message at the Debug level.
//
// Parameters:
//
//   - msg: the debug message.
//
// Example usage:
//
//	logger.Debug("User object created")
func (l *logrusAdapter) Debug(msg string) {
	l.entry.Debug(msg)
}

// Debugf logs a formatted debug message at the Debug level.
//
// Parameters:
//
//   - format: the message format.
//   - args: the arguments for the format.
//
// Example usage:
//
//	logger.Debugf("Response time: %dms", ms)
func (l *logrusAdapter) Debugf(format string, args ...any) {
	l.entry.Debugf(format, args...)
}

// Fatal logs a message at the Fatal level and terminates the application.
//
// Parameters:
//
//   - msg: the fatal error message.
//
// Example usage:
//
//	logger.Fatal("Configuration missing. Exiting.")
func (l *logrusAdapter) Fatal(msg string) {
	l.entry.Fatal(msg)
}

// Fatalf logs a formatted fatal error message at the Fatal level and terminates the application.
//
// Parameters:
//
//   - format: the message format.
//   - args: the arguments for the format.
//
// Example usage:
//
//	logger.Fatalf("Cannot load config file: %s", path)
func (l *logrusAdapter) Fatalf(format string, args ...any) {
	l.entry.Fatalf(format, args...)
}

// Trace logs a message at the Trace level, providing fine-grained debugging information.
//
// Parameters:
//
//   - msg: the trace message.
//
// Example usage:
//
//	logger.Trace("Entered handler function")
func (l *logrusAdapter) Trace(msg string) {
	l.entry.Trace(msg)
}

// Tracef logs a formatted trace message at the Trace level.
//
// Parameters:
//
//   - format: the message format.
//   - args: the arguments for the format.
//
// Example usage:
//
//	logger.Tracef("Payload: %+v", payload)
func (l *logrusAdapter) Tracef(format string, args ...any) {
	l.entry.Tracef(format, args...)
}

// WithField returns a new Logger instance with a single key-value pair added to the log context.
//
// Parameters:
//
//   - key: the context field key.
//   - value: the context field value.
//
// Example usage:
//
//	logger.WithField("user_id", 42).Info("User logged in")
func (l *logrusAdapter) WithField(key string, value any) {
	*l = logrusAdapter{entry: l.entry.WithField(key, value)}
}

// WithFields returns a new Logger instance with multiple key-value pairs added to the log context.
//
// Parameters:
//
//   - fields: a map containing field keys and their corresponding values.
//
// Example usage:
//
//	logger.WithFields(map[string]any{"user_id": 42, "role": "admin"}).Info("Access granted")
func (l *logrusAdapter) WithFields(fields map[string]any) {
	*l = logrusAdapter{entry: l.entry.WithFields(fields)}
}

// WithRequestStringID returns a new Logger instance with a string request ID added to the context.
//
// Parameters:
//
//   - id: the request ID as a string.
//
// Example usage:
//
//	logger.WithRequestStringID("req-123").Info("Request started")
func (l *logrusAdapter) WithRequestStringID(id string) {
	*l = logrusAdapter{entry: l.entry.WithField(KeyRequestID, id)}
}

// WithRequestUUID returns a new Logger instance with a UUID-based request ID added to the context.
//
// Parameters:
//
//   - id: the UUID for the request.
//
// Example usage:
//
//	logger.WithRequestUUID(uuid.New()).Info("Tracking UUID request")
func (l *logrusAdapter) WithRequestUUID(id uuid.UUID) {
	*l = logrusAdapter{entry: l.entry.WithField(KeyRequestID, id)}
}

// WithRequestID returns a new Logger instance with a numeric request ID added to the context.
//
// Parameters:
//
//   - id: the numeric request ID.
//
// Example usage:
//
//	logger.WithRequestID(1001).Info("Handling request")
func (l *logrusAdapter) WithRequestID(id uint64) {
	*l = logrusAdapter{entry: l.entry.WithField(KeyRequestID, id)}
}

// WithRandomRequestID returns a new Logger instance with a randomly generated numeric request ID.
//
// Example usage:
//
//	logger.WithRandomRequestID().Info("Generated random request ID")
func (l *logrusAdapter) WithRandomRequestID() {
	*l = logrusAdapter{entry: l.entry.WithField(KeyRequestID, rand.Uint64())}
}

// WithSystemRequestID returns a new Logger instance with a system configuration ID added to the context.
//
// Parameters:
//
//   - id: the system configuration ID (uint8).
//
// Example usage:
//
//	logger.WithSystemRequestID(3).Info("Using config #3")
func (l *logrusAdapter) WithSystemRequestID(id uint8) {
	*l = logrusAdapter{entry: l.entry.WithField(KeySystemRequestID, id)}
}

// WithUserID returns a new Logger instance with a user ID added to the log context.
//
// Parameters:
//
//   - userID: the user identifier.
//
// Example usage:
//
//	logger.WithUserID(12345).Info("User performed action")
func (l *logrusAdapter) WithUserID(userID uint64) {
	*l = logrusAdapter{entry: l.entry.WithField(KeyUserID, userID)}
}

// GetFields returns the current log context fields as a map.
//
// Returns:
//
//   - map[string]any: a map containing the current log context fields.
func (l *logrusAdapter) GetFields() map[string]any {
	return l.entry.Data
}

// GetField returns the value of a specific field from the log context.
//
// Parameters:
//
//   - key: the field key.
//
// Returns:
//
//   - any: the value of the field. if the field is not found, it returns nil.
//
// Example usage:
//
//	val := logger.GetField("user_id")
//	if val != nil {
//	  // Handle missing field
//	}
func (l *logrusAdapter) GetField(key string) any {
	val, ok := l.entry.Data[key]
	if !ok {
		return nil
	}

	return val
}

// DeleteField removes a field from the current log context.
//
// Parameters:
//
//   - key: the field key to remove.
//
// Example usage:
//
//	logger.DeleteField("user_id")
func (l *logrusAdapter) DeleteField(key string) {
	delete(l.entry.Data, key)
}
