package logger

type Logger interface {
	Info(msg string)
	Infof(format string, args ...any)
	Trace(msg string)
	Tracef(format string, args ...any)
	Error(msg string)
	Errorf(format string, args ...any)
	Warn(msg string)
	Warnf(format string, args ...any)
	Debug(msg string)
	Debugf(format string, args ...any)
	Fatal(msg string)
	Fatalf(format string, args ...any)
	WithField(key string, value any) Logger
	WithFields(fields map[string]any) Logger
	WithRequestRandomID() Logger
	WithSystemConfigID() Logger
	WithUserID(userID uint64) Logger
}
