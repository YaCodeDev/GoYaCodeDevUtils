package yalocales

import "go.uber.org/fx"

// LocalizerParams configures the Localizer provided by LocalizerModule.
type LocalizerParams struct {
	FallbackLang             string
	EnforceLocaleConsistency bool
}

// newLocalizerFromParams builds a Localizer from LocalizerParams.
func newLocalizerFromParams(params LocalizerParams) Localizer {
	return NewLocalizer(params.FallbackLang, params.EnforceLocaleConsistency)
}

// LocalizerModuleName is the fx module name for the yalocales providers.
const LocalizerModuleName = "yalocales"

// LocalizerModule provides a Localizer configured through the LocalizerParams
// supplied by the consuming app.
//
// Example usage:
//
//	fx.New(
//		fx.Supply(yalocales.LocalizerParams{FallbackLang: "en"}),
//		yalocales.LocalizerModule,
//	)
var LocalizerModule = fx.Module(
	LocalizerModuleName,
	fx.Provide(newLocalizerFromParams),
)
