// Package yabackoff provides simple, self-contained back-off strategies for
// retry loops.  A back-off progressively increases the time you wait between
// attempts of an operation that might fail (for example, an HTTP request).
//
// # Quick start
//
//	backoff := yabackoff.NewExponential(500*time.Millisecond, 1.5, 60*time.Second)
//	for {
//	    if err := doWork(); err == nil {
//	        break // success – stop retrying
//	    }
//	    backoff.Wait() // progressively longer sleeps
//	}
//
// The package is dependency-free and can be safely vendored.
package yabackoff

import (
	"time"
)

// Default* constants are applied when the caller provides zero
// values to NewExponential, or when an Exponential is declared
// as a zero value and used without initialisation.
const (
	// DefaultInitialInterval is used when initialInterval == 0.
	DefaultInitialInterval = 500 * time.Millisecond

	// DefaultMultiplier is applied when multiplier == 0.
	DefaultMultiplier = 1.5

	// DefaultMaxInterval is used when maxInterval == 0.
	DefaultMaxInterval = 60 * time.Second
)

// Backoff is the behaviour shared by all back‑off strategies in this package.
// Implementations are *not* safe for concurrent use – surround them with your
// own synchronisation if you share one instance between goroutines.
//
// Example:
//
//	backoff := yabackoff.NewExponential(100*time.Millisecond, 2, time.Second)
//	_ = b.Next() // 200 ms
//	_ = b.Next() // 400 ms
//	b.Reset()    // back to 100 ms
//
// The concrete type behind the interface decides how the delays grow.
type Backoff interface {
	// Next advances the strategy and returns the delay for *this* attempt.
	Next() time.Duration

	// Current returns the delay that was (or will be) produced by the most
	// recent (or next) call to Next().  It never mutates internal state.
	Current() time.Duration

	// Wait is a convenience wrapper that simply does:
	//
	//   time.Sleep(b.Next())
	//
	// Example:
	//
	//   backoff.Wait() // sleeps for the next back‑off interval and updates state
	Wait()

	// Reset puts the strategy back to its initial state so that the very next
	// call to Next() will return the initial interval again.
	//
	// Example:
	//
	//   backoff.Reset()
	Reset()
}
