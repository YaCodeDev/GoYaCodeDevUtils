package logger

type Level uint8

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

type BaseLoggerType uint8

const (
	Logrus BaseLoggerType = iota
)

const (
	KeyRequestID       = "request_id"
	KeySystemRequestID = "system_request_id"
	KeyUserID          = "user_id"
)
