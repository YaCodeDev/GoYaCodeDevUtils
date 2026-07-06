package yabackoff_test

import (
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yabackoff"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestBackoffModule(t *testing.T) {
	t.Parallel()

	t.Run(
		"when BackoffModule is wired with ExponentialParams / then it resolves a Backoff using those params",
		func(t *testing.T) {
			t.Parallel()

			const (
				initialInterval = 100 * time.Millisecond
				multiplier      = 2.0
				maxInterval     = time.Second
				resetAfter      = time.Minute
			)

			var backoff yabackoff.Backoff

			fxtest.New(
				t,
				yabackoff.BackoffModule,
				fx.Supply(yabackoff.ExponentialParams{
					InitialInterval: initialInterval,
					Multiplier:      multiplier,
					MaxInterval:     maxInterval,
					ResetAfter:      resetAfter,
				}),
				fx.Populate(&backoff),
			)

			if backoff == nil {
				t.Fatalf("expected BackoffModule to populate a non-nil Backoff")
			}

			if got := backoff.Current(); got != initialInterval {
				t.Errorf("expected initial Current() to equal %s, got %s", initialInterval, got)
			}
		},
	)
}
