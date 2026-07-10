package yasmtp

import "go.uber.org/fx"

// ModuleName is the fx module name for the yasmtp mailer.
const ModuleName = "yasmtp"

// Module provides a *Mailer from a supplied *Config.
//
// Example usage:
//
//	fx.New(
//		fx.Supply(&yasmtp.Config{
//			Host: "mail.example.com",
//			Port: 587,
//			From: "noreply@example.com",
//		}),
//		yalogger.LoggerModule,
//		yasmtp.Module,
//	)
var Module = fx.Module(
	ModuleName,
	fx.Provide(NewMailer),
)
