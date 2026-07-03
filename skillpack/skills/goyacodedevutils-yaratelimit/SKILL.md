---
name: goyacodedevutils-yaratelimit
description: Fixed-window rate limiter backed by any yacache.Cache, keyed by (id, group). Use instead of hand-rolling a request-counting rate limit.
---

# yaratelimit Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yaratelimit`.

Fixed-window rate limiter backed by any `yacache.Cache`, keyed by `(id uint64, group string)`.

## Key API

- `IRateLimit` interface — `CheckBanned`, `Refresh`, `Increment`, `Get`.
- `RateLimit[Cache yacache.Container]` struct — `{ Cache, Limit uint8, Rate time.Duration }`; `NewRateLimit[Cache](cache yacache.Cache[Cache], limit uint8, rate time.Duration) *RateLimit[Cache]`.
- `Storage` struct — `{ Limit uint8 (current count), FirstRequest int64 }`.
- `FormatKey(id uint64, group string) string`, `FormatValue(limit uint8, firstRequest int64) string`.

## Usage Notes

- `Increment(ctx, id, group)` returns `banned bool = true` once the count reaches/exceeds `Limit` within the current `Rate`-duration window; the window resets automatically once `Rate` has elapsed since the first hit.
- `CheckBanned` is a read-only pre-check (does not increment) — use it before an expensive operation, then call `Increment` to record the attempt.
- Depends on `yacache` + `yaerrors`. Storage value is a raw CSV string `"<count>,<first_unix_sec>"` cached at key `"rate-limit-<id>-<group>"` with no TTL — staleness comes from the window logic, not cache expiry.
