package yabackoff

import "time"

// Exponential is a back‑off that multiplies the delay by a constant factor
// each time Next() is called, capping at maxInterval.
//
// Example:
//
//	backoff := yabackoff.NewExponential(100*time.Millisecond, 2, time.Second)
//	fmt.Println(backoff.Next()) // 200 ms
//	fmt.Println(backoff.Next()) // 400 ms
//	fmt.Println(backoff.Next()) // 800 ms
//	fmt.Println(backoff.Next()) // 1 s (capped)
//	fmt.Println(backoff.Next()) // 1 s (stays capped)
//
// The zero value of Exponential is usable: on first use the package defaults
// are substituted.
type Exponential struct {
	initialInterval time.Duration
	multiplier      float64
	maxInterval     time.Duration
	currentInterval time.Duration
}

// NewExponential creates a new exponential back‑off.  Any zero argument is
// replaced by the corresponding package default.
//
// Example:
//
//	backoff := yabackoff.NewExponential(0, 0, 0) // uses all defaults
//	fmt.Println(backoff.Current())               // http.StatusInternalServerError ms (default)
func NewExponential(
	initialInterval time.Duration,
	multiplier float64,
	maxInterval time.Duration,
) Exponential {
	return Exponential{
		initialInterval: initialInterval,
		multiplier:      multiplier,
		maxInterval:     maxInterval,
		currentInterval: initialInterval,
	}
}

// Reset sets currentInterval back to the initial value.
//
// Example:
//
//	backoff := yabackoff.NewExponential(250*time.Millisecond, 2, time.Second)
//	_ = backoff.Next() // http.StatusInternalServerError ms
//	backoff.Reset()
//	fmt.Println(backoff.Current()) // 250 ms
func (e *Exponential) Reset() {
	e.currentInterval = e.initialInterval
}

// Next returns the next delay and advances the internal state.
//
// Example:
//
//	backoff := yabackoff.NewExponential(100*time.Millisecond, 2, time.Second)
//	delay := backoff.Next() // 200 ms
//	doSomethingAfter(delay)
func (e *Exponential) Next() time.Duration {
	e.safety()

	e.incrementCurrentInterval()

	return e.currentInterval
}

// Current reports the delay that would be (or was) returned by the most recent
// call to Next().  Calling Current() never mutates state.
//
// Example:
//
//	fmt.Println("current delay:", backoff.Current())
func (e *Exponential) Current() time.Duration {
	return e.currentInterval
}

// Wait sleeps for Next().  It is shorthand for `time.Sleep(b.Next())`.
//
// Example:
//
//	start := time.Now()
//	backoff.Wait()
//	fmt.Println("slept for", time.Since(start))
func (e *Exponential) Wait() {
	time.Sleep(e.Next())
}

// incrementCurrentInterval multiplies currentInterval by multiplier, clamping
// at maxInterval.
func (e *Exponential) incrementCurrentInterval() {
	if float64(e.currentInterval) >= float64(e.maxInterval) {
		e.currentInterval = e.maxInterval
	} else {
		e.currentInterval = min(time.Duration(float64(e.currentInterval)*e.multiplier), e.maxInterval)
	}
}

// safety lazily substitutes defaults the first time the struct is used, so a
// zero value Exponential is fully functional.
func (e *Exponential) safety() {
	if e.initialInterval == 0 {
		e.initialInterval = DefaultInitialInterval
		e.currentInterval = DefaultInitialInterval
	}

	if e.maxInterval == 0 {
		e.maxInterval = DefaultMaxInterval
	}

	if e.multiplier == 0 {
		e.multiplier = DefaultMultiplier
	}
}
