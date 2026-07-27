package yaturnstile

import "go.uber.org/fx"

// ModuleName is the fx module name for the yaturnstile verifier.
const ModuleName = "yaturnstile"

// Module provides a Verifier from a supplied *Config.
//
// Example usage:
//
//	fx.New(
//		fx.Supply(&yaturnstile.Config{
//			SecretKey: "0x4AAA...",
//			Endpoint:  yaturnstile.DefaultEndpoint,
//		}),
//		yalogger.LoggerModule,
//		yaturnstile.Module,
//	)
var Module = fx.Module(
	ModuleName,
	fx.Provide(NewVerifier),
)
