package yalogger

import "strings"

func (l *Level) String() string {
	switch *l {
	case PanicLevel:
		return "Panic"
	case FatalLevel:
		return "Fatal"
	case ErrorLevel:
		return "Error"
	case WarnLevel:
		return "Warn"
	case InfoLevel:
		return "Info"
	case DebugLevel:
		return "Debug"
	case TraceLevel:
		return "Trace"
	default:
		return "Unknown"
	}
}

func (l *Level) Unmarshal(text string) error {
	switch strings.ToLower(text) {
	case "panic":
		*l = PanicLevel
	case "fatal":
		*l = FatalLevel
	case "error":
		*l = ErrorLevel
	case "warn":
		*l = WarnLevel
	case "info":
		*l = InfoLevel
	case "debug":
		*l = DebugLevel
	case "trace":
		*l = TraceLevel
	default:
		return ErrInvalidLogLevel
	}

	return nil
}

func (l *Level) UnmarshalText(text []byte) error {
	return l.Unmarshal(string(text))
}
