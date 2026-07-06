---
name: goyacodedevutils-yathreadsafeset
description: Generic mutex-protected set[K] with union/difference/intersect/symmetric-difference. Use instead of a map[K]struct{} guarded by a hand-rolled mutex.
---

# yathreadsafeset Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yathreadsafeset`.

Generic mutex-protected set (a `map[K]struct{}` wrapper) with set-algebra operations.

## Key API

- `ThreadSafeSet[K comparable]` struct; `NewThreadSafeSet[K]() *ThreadSafeSet[K]`.
- Methods: `Clear`, `Copy`/`CopyRaw`, `Delete`/`DeleteMultiple`, `Has`, `Iterate`/`IterateOnCopy`/`IterateWithBreak`, `Length`, `IsEmpty`, `MarshalJSON`, `Pop`, `Set`, `ImportFromMap(map[K]struct{})`, `String`, `Values`, `IsEqual(other)`, `Union(other)`, `Difference(other)`, `SymmetricDifference(other)`, `Intersect(other)`.

## Usage Notes

- Thread-safe via `sync.RWMutex`. `Iterate`/`IterateWithBreak` hold the read lock during the callback — mutating the same set inside the callback deadlocks; use `IterateOnCopy` instead.
- `Union`/`Difference`/`Intersect`/`SymmetricDifference` return **new** sets built from snapshots, so callers can mutate either source set after the call without changing the result.
- No dependency on other repo packages; used internally by `yatgstorage` to track lazily-initialized RedisJSON state keys.
