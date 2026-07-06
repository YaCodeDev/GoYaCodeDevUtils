package yalogger

import "go.uber.org/fx"

// LoggerModuleName is the fx module name for the yalogger providers.
const LoggerModuleName = "yalogger"

// LoggerModule provides a BaseLogger built from the *Config supplied by the
// consuming app, plus a ready-to-use Logger derived from it.
//
// Example usage:
//
//	fx.New(
//		fx.Supply(&yalogger.Config{}),
//		yalogger.LoggerModule,
//	)
var LoggerModule = fx.Module(
	LoggerModuleName,
	fx.Provide(NewBaseLogger),
	fx.Provide(newLoggerFromBase),
)

// newLoggerFromBase derives a Logger from the BaseLogger this module provides.
func newLoggerFromBase(base BaseLogger) Logger {
	return base.NewLogger()
}
