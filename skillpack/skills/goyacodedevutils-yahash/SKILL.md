---
name: goyacodedevutils-yahash
description: Generic helper to build salted, optionally time-windowed hashes (e.g. short-lived tokens or request signatures) around any hash function. Use instead of hand-rolling HMAC/token expiry logic.
---

# yahash Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yahash`.

Generic helper to build salted, optionally time-windowed hashes (e.g. short-lived tokens or request
signatures) using any user-supplied hash function.

## Key API

- `HashableType = valueparser.ParsableType`.
- `HashFunc[I HashableType, O comparable] func(data I, args ...I) O`.
- `Hash[I, O]` struct; `NewHash[I, O](hasher HashFunc[I, O], secret I, stepInterval time.Duration, backStepCount uint16) Hash[I, O]`.
- Methods: `Hash(data I, args ...I) O`, `HashWithTime(t time.Time, args ...I) O`, `ValidateWithoutTime(expected O, data I, args ...I) bool`, `Validate(expected O, args ...I) bool`, `ValidateWithCustomBackStepCount(expected O, backStepCount uint16, args ...I) bool`.
- Ready-made hash funcs: `FNVStringToInt64(data string, args ...string) int64`, `FNVStringToInt32(...) int32`.

## Usage Notes

- `secret` is auto-appended as the last arg to every `Hash()`/`HashWithTime()` call — do not pass it manually.
- `stepInterval < 1s` is silently promoted to `1s`. `Validate` checks the current time window plus `backStepCount` previous windows, for clock-drift/latency tolerance.
- Depends on `valueparser` (for `HashWithTime`'s timestamp-to-`I` conversion). The zero value is **not** usable — always construct with `NewHash`.
