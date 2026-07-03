---
name: goyacodedevutils-threadsafemap
description: Generic mutex-protected map[K]V with convenience methods for concurrent access. Use instead of a raw map guarded by a hand-rolled sync.Mutex/sync.RWMutex.
---

# threadsafemap Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/threadsafemap`.

Generic mutex-protected `map[K]V` with convenience methods for concurrent access.

## Key API

- `ThreadSafeMap[K comparable, V any]` struct; `NewThreadSafeMap[K, V]() *ThreadSafeMap[K, V]`.
- Methods: `Clear`, `Copy`, `Delete`, `Get`, `GetOrDefault`, `GetOrSet`, `Has`, `Iterate`, `IterateOnCopy`, `IterateWithBreak`, `Keys`, `Length`, `MarshalJSON`, `Pop`, `Set`, `String`, `Update(key, fn(old V, exists bool) V)`, `Values`.

## Usage Notes

- Fully thread-safe via `sync.RWMutex`. Always construct with `NewThreadSafeMap` rather than a bare zero-value struct.
- `Iterate`/`IterateWithBreak` hold the read lock for the whole callback — do not call `Set`/`Delete` on the same map inside the callback (deadlock). Use `IterateOnCopy` if you need to mutate during iteration.
- No dependency on other repo packages.
