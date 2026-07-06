package yabackoff

import (
	"time"

	"go.uber.org/fx"
)

// ExponentialParams configures the Exponential back-off provided by
// BackoffModule. A zero value for any field falls back to the corresponding
// yabackoff default, mirroring NewExponential's own zero-value behavior.
type ExponentialParams struct {
	InitialInterval time.Duration
	Multiplier      float64
	MaxInterval     time.Duration
	ResetAfter      time.Duration
}

// newExponentialBackoff builds an *Exponential from ExponentialParams and
// exposes it as a Backoff for fx.
func newExponentialBackoff(params ExponentialParams) *Exponential {
	backoff := NewExponential(
		params.InitialInterval,
		params.Multiplier,
		params.MaxInterval,
		params.ResetAfter,
	)

	return &backoff
}

// BackoffModuleName is the fx module name for the yabackoff providers.
const BackoffModuleName = "yabackoff"

// BackoffModule provides a Backoff backed by Exponential, configured through
// the ExponentialParams supplied by the consuming app.
//
// Example usage:
//
//	fx.New(
//		fx.Supply(yabackoff.ExponentialParams{
//			InitialInterval: 500 * time.Millisecond,
//			Multiplier:      1.5,
//			MaxInterval:     60 * time.Second,
//		}),
//		yabackoff.BackoffModule,
//	)
var BackoffModule = fx.Module(
	BackoffModuleName,
	fx.Provide(fx.Annotate(newExponentialBackoff, fx.As(new(Backoff)))),
)
