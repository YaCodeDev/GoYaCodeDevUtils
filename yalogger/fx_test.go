package yalogger_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestLoggerModule(t *testing.T) {
	t.Parallel()

	t.Run(
		"when LoggerModule is wired with a nil *Config / then it resolves BaseLogger and Logger",
		func(t *testing.T) {
			t.Parallel()

			var (
				base   yalogger.BaseLogger
				logger yalogger.Logger
			)

			fxtest.New(
				t,
				yalogger.LoggerModule,
				fx.Supply((*yalogger.Config)(nil)),
				fx.Populate(&base, &logger),
			)

			if base == nil {
				t.Errorf("expected LoggerModule to populate a non-nil BaseLogger")
			}

			if logger == nil {
				t.Errorf("expected LoggerModule to populate a non-nil Logger")
			}
		},
	)
}
