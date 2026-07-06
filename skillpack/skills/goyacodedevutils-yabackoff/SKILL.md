---
name: goyacodedevutils-yabackoff
description: Simple exponential back-off strategy for retry loops. Use instead of a hand-rolled sleep-and-double retry loop.
---

# yabackoff Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yabackoff`.

Exponential back-off strategy for retry loops.

## Key API

- `Backoff` interface — `Next() time.Duration`, `Current() time.Duration`, `Wait()`, `Reset()`.
- `Exponential` struct (implements `Backoff`); `NewExponential(initialInterval, multiplier, maxInterval, resetAfter time.Duration) Exponential`.
- `const DefaultInitialInterval = 500ms`, `DefaultMultiplier = 1.5`, `DefaultMaxInterval = 60s`.

## Usage Notes

- A zero-value `Exponential` is usable: missing/zero constructor args are lazily replaced with the package defaults on first `Next()`/`Current()` call.
- `resetAfter` (4th constructor arg): if the time since the last `Next()` call exceeds it, the interval auto-resets to the initial value; pass `0` to disable auto-reset.
- Not concurrency-safe — use one instance per goroutine, or add your own locking. No dependency on other repo packages; used internally by `yatgclient` for reconnect backoff.
- Fx: `BackoffModule` (`fx.go`) provides a `Backoff` from a supplied `ExponentialParams`.
