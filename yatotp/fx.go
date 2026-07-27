package yatotp

import "go.uber.org/fx"

// ModuleName is the fx module name for the yatotp authenticator.
const ModuleName = "yatotp"

// Module provides an *Authenticator from a supplied *Config.
//
// Example usage:
//
//	fx.New(
//		fx.Supply(&yatotp.Config{Issuer: "Example"}),
//		yatotp.Module,
//	)
var Module = fx.Module(
	ModuleName,
	fx.Provide(NewAuthenticator),
)
