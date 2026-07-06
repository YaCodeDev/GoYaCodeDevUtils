package yalocales_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalocales"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestLocalizerModule(t *testing.T) {
	t.Parallel()

	t.Run(
		"when LocalizerModule is wired with LocalizerParams / then it resolves a Localizer using those params",
		func(t *testing.T) {
			t.Parallel()

			const missingKey = "fx-wiring-missing-key"

			var localizer yalocales.Localizer

			fxtest.New(
				t,
				yalocales.LocalizerModule,
				fx.Supply(yalocales.LocalizerParams{
					FallbackLang:             "en",
					EnforceLocaleConsistency: true,
				}),
				fx.Populate(&localizer),
			)

			if localizer == nil {
				t.Fatalf("expected LocalizerModule to populate a non-nil Localizer")
			}

			if _, err := localizer.GetDefaultLangValueByCompositeKey(missingKey); err == nil {
				t.Errorf("expected lookup of an unloaded key to fail")
			}
		},
	)
}
