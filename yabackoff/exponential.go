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
	lastWaitEnd     time.Time
	resetAfter      time.Duration
}

// NewExponential creates a new exponential back‑off. Any zero argument is
// replaced by the corresponding package default.
//
// Example:
//
//	backoff := yabackoff.NewExponential(0, 0, 0, 0)	// uses all defaults
//	fmt.Println(backoff.Current())               	// 500 ms (default)
func NewExponential(
	initialInterval time.Duration,
	multiplier float64,
	maxInterval time.Duration,
	resetAfter time.Duration,
) Exponential {
	return Exponential{
		initialInterval: initialInterval,
		multiplier:      multiplier,
		maxInterval:     maxInterval,
		resetAfter:      resetAfter,
	}
}

// Reset sets currentInterval back to the initial value.
//
// Example:
//
//	backoff := yabackoff.NewExponential(250*time.Millisecond, 2, time.Second, 0)
//	_ = backoff.Next() // 500 ms
//	backoff.Reset()
//	fmt.Println(backoff.Current()) // 250 ms
func (e *Exponential) Reset() {
	e.currentInterval = 0
	e.lastWaitEnd = time.Time{}
}

// Next returns the next delay and advances the internal state.
//
// Example:
//
//	backoff := yabackoff.NewExponential(100*time.Millisecond, 2, time.Second, 0)
//	delay := backoff.Next() // 200 ms
//	doSomethingAfter(delay)
func (e *Exponential) Next() time.Duration {
	e.safety()

	e.incrementCurrentInterval()

	return e.currentInterval
}

// Current reports the delay that would be (or was) returned by the most recent
// call to Next(). Calling Current() never mutates state.
//
// Example:
//
//	fmt.Println("current delay:", backoff.Current())
func (e *Exponential) Current() time.Duration {
	if e.currentInterval == 0 {
		e.safety()

		return e.initialInterval
	}

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

// autoReset resets the back‑off if the time since the last wait exceeds resetAfter.
func (e *Exponential) autoReset() {
	if e.resetAfter == 0 {
		return
	}

	if e.currentInterval != e.initialInterval && time.Since(e.lastWaitEnd) >= e.resetAfter {
		e.Reset()
	}
}

// incrementCurrentInterval multiplies currentInterval by multiplier, clamping
// at maxInterval.
func (e *Exponential) incrementCurrentInterval() {
	e.autoReset()

	switch {
	case e.currentInterval == 0:
		e.currentInterval = e.initialInterval
	case float64(e.currentInterval) >= float64(e.maxInterval):
		e.currentInterval = e.maxInterval
	default:
		e.currentInterval = min(
			time.Duration(float64(e.currentInterval)*e.multiplier),
			e.maxInterval,
		)
	}

	e.lastWaitEnd = time.Now().Add(e.currentInterval)
}

// safety lazily substitutes defaults the first time the struct is used, so a
// zero value Exponential is fully functional.
func (e *Exponential) safety() {
	if e.initialInterval == 0 {
		e.initialInterval = DefaultInitialInterval
	}

	if e.maxInterval == 0 {
		e.maxInterval = DefaultMaxInterval
	}

	if e.multiplier == 0 {
		e.multiplier = DefaultMultiplier
	}
}
